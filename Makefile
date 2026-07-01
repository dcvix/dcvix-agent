MAIN_NAME=dcvix-agent

DIST_DIR=dist

# Version information
VERSION?=$(shell (git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0") | sed 's/^v//')
VERSION_PARTS=$(subst ., ,$(VERSION))
VERSION_MAJOR=$(or $(word 1,$(VERSION_PARTS)),0)
VERSION_MINOR=$(or $(word 2,$(VERSION_PARTS)),0)
VERSION_PATCH=$(or $(word 3,$(VERSION_PARTS)),0)
RELEASE=1
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build variables
BINARY_NAME=$(MAIN_NAME)
GO=$(shell which go)
LDFLAGS="-X github.com/dcvix/$(MAIN_NAME)/internal/version.Version=$(VERSION) \
         -X github.com/dcvix/$(MAIN_NAME)/internal/version.Commit=$(COMMIT) \
         -X github.com/dcvix/$(MAIN_NAME)/internal/version.BuildTime=$(BUILD_TIME)"

# Platform-specific variables
LINUX_AMD64_BINARY=$(MAIN_NAME)
LINUX_AMD64_DIR=$(MAIN_NAME)-v$(VERSION)-linux-amd64
WINDOWS_AMD64_BINARY=$(MAIN_NAME).exe
WINDOWS_AMD64_DIR=$(MAIN_NAME)-v$(VERSION)-windows_amd64

# Windows installer variables
NSIS=makensis
NSIS_SCRIPT=contrib/nsis/installer.nsi
INSTALLER_NAME=$(BINARY_NAME)-v$(VERSION)-setup.exe

# RPM variables
RPM_NAME=$(BINARY_NAME)
RPM_VERSION=$(VERSION)
RPM_RELEASE=$(RELEASE)
RPM_TOPDIR=$(CURDIR)/rpmbuild
RPM_SOURCES=$(RPM_TOPDIR)/SOURCES
RPM_SPECS=$(RPM_TOPDIR)/SPECS

# Debian package variables
DEB_NAME=$(BINARY_NAME)
DEB_VERSION=$(VERSION)
DEB_RELEASE=$(RELEASE)
DEB_TOPDIR=$(CURDIR)/debbuild
DEB_SOURCE=$(DEB_TOPDIR)/$(DEB_NAME)-$(DEB_VERSION)
DEB_CONTROL=$(DEB_SOURCE)/DEBIAN
DEB_BINARY=$(DEB_SOURCE)/usr/bin
DEB_CONFIG=$(DEB_SOURCE)/etc/$(DEB_NAME)
DEB_SYSTEMD=$(DEB_SOURCE)/etc/systemd/system
DEB_LOG=$(DEB_SOURCE)/var/log/$(DEB_NAME)
DEB_DOC=$(DEB_SOURCE)/usr/share/doc/$(DEB_NAME)

# Build all platforms
.PHONY: build
build: build-linux build-windows

# Build for Linux
.PHONY: build-linux
build-linux:
	mkdir -p $(DIST_DIR)/$(LINUX_AMD64_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build \
		-trimpath -ldflags $(LDFLAGS) \
		-o $(DIST_DIR)/$(LINUX_AMD64_DIR)/$(LINUX_AMD64_BINARY) ./cmd/$(MAIN_NAME)
	cp README.md LICENSE.md $(DIST_DIR)/$(LINUX_AMD64_DIR)/
	cd $(DIST_DIR) && tar czf $(LINUX_AMD64_DIR).tar.gz $(LINUX_AMD64_DIR)

# Build for Windows
.PHONY: build-windows
build-windows: windows-resource
	mkdir -p $(DIST_DIR)/$(WINDOWS_AMD64_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build \
		-trimpath -ldflags $(LDFLAGS) \
		-o $(DIST_DIR)/$(WINDOWS_AMD64_DIR)/$(WINDOWS_AMD64_BINARY) ./cmd/$(MAIN_NAME)
	cp README.md LICENSE.md $(DIST_DIR)/$(WINDOWS_AMD64_DIR)/
	cd $(DIST_DIR) && 7z a -bd -r $(WINDOWS_AMD64_DIR).zip $(WINDOWS_AMD64_DIR)
	# Clean up generated resource files
	rm -f cmd/$(MAIN_NAME)/*.syso

# Compile resource file for version info and icon
.PHONY: windows-resource
windows-resource:
	go-winres simply \
		--product-version $(VERSION).0 \
		--file-version $(VERSION).0 \
		--file-description "A REST API service written in Go that manages local Amazon DCV sessions and reports system statistics to a director." \
		--product-name "dcvix Agent" \
		--copyright "Diego Cortassa" \
		--original-filename "$(WINDOWS_AMD64_BINARY)" \
		--icon contrib/windows/dcvix-agent.ico
	mv *.syso cmd/$(MAIN_NAME)/

## audit: run quality control checks
.PHONY: audit
audit:
	$(GO) mod tidy -diff
	$(GO) mod verify
	test -z "$(shell gofmt -l .)"
	$(GO) vet ./...
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest -checks=all ./...
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Show version
.PHONY: version
version:
	@echo $(VERSION)

# Clean build artifacts
.PHONY: clean
clean: installer-clean rpm-clean deb-clean
	rm -rf $(DIST_DIR)

# Run the application
.PHONY: run
run: build
	./$(DIST_DIR)/$(LINUX_AMD64_DIR)/$(LINUX_AMD64_BINARY)

# Run tests
.PHONY: test
test:
	$(GO) test ./... ;

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod tidy ;
	$(GO) mod verify ;

################# Manual INSTALLATION ################

# Install the application
.PHONY: install
install: build-linux
	mkdir -p /etc/dcvix-agent/certs
	install -m 755 $(DIST_DIR)/$(LINUX_AMD64_DIR)/$(LINUX_AMD64_BINARY) /usr/bin/$(BINARY_NAME)
	install -m 644 internal/config/dcvix-agent.conf.default /etc/dcvix-agent/dcvix-agent.conf
	# Change log directory to /var/log/dcvix-agent
	sed -i 's|directory = log|directory = /var/log/dcvix-agent|' /etc/dcvix-agent/dcvix-agent.conf
	install -m 644 contrib/systemd/dcvix-agent.service /var/lib/systemd/system/
	mkdir -p /var/log/dcvix-agent
	systemctl daemon-reload
	@echo "Installation complete."
	@echo "Edit /etc/dcvix-agent/dcvix-agent.conf and start the service with:"
	@echo "systemctl enable dcvix-agent"
	@echo "systemctl start dcvix-agent"

.PHONY: uninstall
uninstall:
	systemctl stop dcvix-agent
	systemctl disable dcvix-agent
	rm -f /usr/bin/$(BINARY_NAME)
	rm -rf /etc/dcvix-agent
	rm -f /var/lib/systemd/system/dcvix-agent.service
	rm -rf /var/log/dcvix-agent
	systemctl daemon-reload
	@echo "Uninstallation complete."

################# RPM PACKAGE #################

# Build RPM package
.PHONY: rpm
rpm: rpm-prep
	cp contrib/rpm/$(RPM_NAME).spec $(RPM_SPECS)/
	rpmbuild --define "_topdir $(RPM_TOPDIR)" --define "pkg_version $(RPM_VERSION)" -ba $(RPM_SPECS)/$(RPM_NAME).spec
	mkdir -p $(DIST_DIR)
	cp $(RPM_TOPDIR)/RPMS/*/*.rpm $(DIST_DIR)/
	cp $(RPM_TOPDIR)/SRPMS/*.rpm $(DIST_DIR)/

# Prepare source for RPM
.PHONY: rpm-prep
rpm-prep:
	mkdir -p $(RPM_SOURCES)
	mkdir -p $(RPM_SPECS)
	cp -r README.md LICENSE.md Makefile cmd/ contrib/ internal/ $(RPM_SOURCES)/
	# Change log directory to /var/log/dcvix-agent
	sed 's|directory = log|directory = /var/log/dcvix-agent|' internal/config/dcvix-agent.conf.default > $(RPM_SOURCES)/dcvix-agent.conf
	tar --transform 's,^,$(RPM_NAME)-$(RPM_VERSION)/,' -C $(RPM_SOURCES) -czf \
		$(RPM_SOURCES)/$(RPM_NAME)-$(RPM_VERSION).tar.gz \
		README.md LICENSE.md Makefile dcvix-agent.conf cmd/ contrib/ internal/

# Clean RPM build artifacts
.PHONY: rpm-clean
rpm-clean:
	rm -rf $(RPM_TOPDIR)

################# DEBIAN PACKAGE #################

# Build Debian package
.PHONY: deb
deb: deb-prep
	cd $(DEB_TOPDIR) && dpkg-deb --build $(DEB_NAME)-$(DEB_VERSION)
	mv $(DEB_TOPDIR)/$(DEB_NAME)-$(DEB_VERSION).deb $(DIST_DIR)/

# Prepare source for Debian package
.PHONY: deb-prep
deb-prep: build-linux
	mkdir -p $(DEB_CONTROL)
	mkdir -p $(DEB_BINARY)
	mkdir -p $(DEB_CONFIG)
	mkdir -p $(DEB_SYSTEMD)
	mkdir -p $(DEB_LOG)
	mkdir -p $(DEB_DOC)
	# Copy binary
	cp $(DIST_DIR)/$(LINUX_AMD64_DIR)/$(BINARY_NAME) $(DEB_BINARY)/
	# Copy config and  change log directory to /var/log/dcvix-agent
	sed 's|directory = log|directory = /var/log/dcvix-agent|' internal/config/dcvix-agent.conf.default > $(DEB_CONFIG)/dcvix-agent.conf
	# Copy systemd service
	cp contrib/systemd/$(BINARY_NAME).service $(DEB_SYSTEMD)/
	# Copy documentation
	cp README.md $(DEB_DOC)/
	cp LICENSE.md $(DEB_DOC)/
	# Process and copy Debian control files
	sed -e 's/@PACKAGE@/$(DEB_NAME)/g' -e 's/@VERSION@/$(DEB_VERSION)-$(DEB_RELEASE)/g' \
		contrib/deb/control.in > $(DEB_CONTROL)/control
	cp contrib/deb/copyright $(DEB_CONTROL)/
	cp contrib/deb/postinst $(DEB_CONTROL)/
	cp contrib/deb/prerm $(DEB_CONTROL)/
	cp contrib/deb/postrm $(DEB_CONTROL)/
	# Make scripts executable
	chmod 755 $(DEB_CONTROL)/postinst
	chmod 755 $(DEB_CONTROL)/prerm
	chmod 755 $(DEB_CONTROL)/postrm

# Clean Debian build artifacts
.PHONY: deb-clean
deb-clean:
	rm -rf $(DEB_TOPDIR)

################# WINDOWS INSTALLER #################

# Build Windows installer
.PHONY: installer
installer: build-windows
	mkdir -p $(DIST_DIR)
	makensis -DVERSION=$(VERSION) -DNAME=$(MAIN_NAME) -DSRCDIR="$(CURDIR)" contrib/nsis/installer.nsi

# Clean Windows installer artifacts
.PHONY: installer-clean
installer-clean:
	rm -f $(DIST_DIR)/$(INSTALLER_NAME)
