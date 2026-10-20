GCM_SECRET?=
.PHONY: test
test:  ## Run export unit tests that will use GCM if GCM_SECRET is present.
ifneq ($(GCM_SECRET),)
	go test -tags gcme2e -v ./...
else
	@echo "Secret not provided, skipping!"
endif
