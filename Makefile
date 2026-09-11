NODE ?= node

.PHONY: setup env dev build build-go build-web check

setup env dev build build-go build-web check:
	$(NODE) scripts/tasks.mjs $@
