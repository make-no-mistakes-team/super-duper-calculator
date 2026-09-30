NODE ?= node

.PHONY: setup env dev build build-go build-web check check-browser

setup env dev build build-go build-web check check-browser:
	$(NODE) scripts/tasks.mjs $@
