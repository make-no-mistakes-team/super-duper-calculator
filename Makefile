NODE ?= node

.PHONY: setup env db down db-reset dev build build-go build-web check

setup env db down db-reset dev build build-go build-web check:
	$(NODE) scripts/tasks.mjs $@
