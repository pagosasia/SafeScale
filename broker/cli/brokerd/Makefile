GO?=go

ifeq ($(OS),Windows_NT)
	EXEC:=brokerd.exe
	EXEC-COVER:=brokerd-cover.exe
else
	EXEC:=brokerd
	EXEC-COVER:=brokerd-cover
endif

BUILD_DATE := `date +%Y-%m-%d\ %H:%M`
VERSIONFILE := version.go

.PHONY:	vet clean

default: all

vet:
	@$(GO) vet ./...

all: clean gensrc
	@$(GO) build -o $(EXEC)
	@$(GO) test -o $(EXEC-COVER) -covermode=count -coverpkg=github.com/CS-SI/SafeScale/... >/dev/null 2>&1

clean:
	@$(RM) $(EXEC-COVER) || true
	@$(RM) $(EXEC) || true

gensrc:
	@$(RM) $(VERSIONFILE) || true
	@echo "package main" > $(VERSIONFILE)
	@echo "const (" >> $(VERSIONFILE)
	@echo "  VERSION = \"0.1.1\"" >> $(VERSIONFILE)
	@echo "  BUILD_DATE = \"$(BUILD_DATE)\"" >> $(VERSIONFILE)
	@echo ")" >> $(VERSIONFILE)