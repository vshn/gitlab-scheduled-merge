IMG_TAG ?= latest

CURDIR ?= $(shell pwd)
BIN_FILENAME ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/gitlab-scheduled-merge

KUSTOMIZE ?= go run sigs.k8s.io/kustomize/kustomize/v5

# Image URL to use all building/pushing image targets
GHCR_IMG ?= ghcr.io/vshn/gitlab-scheduled-merge:$(IMG_TAG)
