NAME = idbvmware

doc:
	godoc . | head -n -3 > README

build:
	go install

exampleconfig: build
	$(GOPATH)/bin/$(NAME) -example

distfile: build exampleconfig doc
	$(eval VERSION := $(shell $(GOPATH)/bin/$(NAME) -version))
	rm -rf /tmp/bytemine-$(NAME)-$(VERSION)
	mkdir /tmp/bytemine-$(NAME)-$(VERSION)
	cp $(GOPATH)/bin/$(NAME) /tmp/bytemine-$(NAME)-$(VERSION)/$(NAME)
	cp idbvmware.json.example /tmp/bytemine-$(NAME)-$(VERSION)/
	cp README /tmp/bytemine-$(NAME)-$(VERSION)/
	cd /tmp && tar czfv /tmp/bytemine-$(NAME)-$(VERSION).tgz bytemine-$(NAME)-$(VERSION)/
	sha256sum /tmp/bytemine-$(NAME)-$(VERSION).tgz
