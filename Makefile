NAME = bytemine-idbvmware

doc:
	godoc . | head -n -3 > README

build:
	go install

exampleconfig: build
	$(GOPATH)/bin/idbvmware -example

distfile: build exampleconfig doc
	$(eval VERSION := $(shell $(GOPATH)/bin/idbvmware -version))
	rm -rf /tmp/$(NAME)-$(VERSION)
	mkdir /tmp/$(NAME)-$(VERSION)
	cp $(GOPATH)/bin/idbvmware /tmp/$(NAME)-$(VERSION)/$(NAME)
	cp idbvmware.json.example /tmp/$(NAME)-$(VERSION)/
	cp README /tmp/$(NAME)-$(VERSION)/
	cd /tmp && tar czfv /tmp/$(NAME)-$(VERSION).tgz \
		$(NAME)-$(VERSION)/
	sha256sum /tmp/$(NAME)-$(VERSION).tgz
