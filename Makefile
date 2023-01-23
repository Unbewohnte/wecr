SRCDIR:=src
EXE:=wecr
TESTDIR:=testing
RELEASEDIR:=release
LICENSE:=COPYING
README:=README.md
UTILITIESDIR:=utilities
UTILITYEXTRACTOR:=extractor
BUILDDIR:=build

LINUXDIR:=$(EXE)_linux
WINDIR:=$(EXE)_windows
DARWINDIR:=$(EXE)_darwin

LINUXDIR32:=$(LINUXDIR)_x32
WINDIR32:=$(WINDIR)_x32

LINUXDIR64:=$(LINUXDIR)_x64
WINDIR64:=$(WINDIR)_x64
DARWINDIR64:=$(DARWINDIR)_x64


all:
	cd $(SRCDIR) && go build && mv $(EXE) ..

test: all
	rm -rf $(TESTDIR) && \
	mkdir -p $(TESTDIR) && \
	cp $(EXE) $(TESTDIR) && \
	cp conf.json $(TESTDIR)

clean:
	rm -rf $(TESTDIR) $(EXE) $(RELEASEDIR)

release: clean
	rm -rf $(RELEASEDIR)

	mkdir -p $(RELEASEDIR)/$(LINUXDIR64)
	mkdir -p $(RELEASEDIR)/$(WINDIR64)
	mkdir -p $(RELEASEDIR)/$(DARWINDIR64)

	cp $(LICENSE) $(RELEASEDIR)/$(LINUXDIR64)
	cp $(LICENSE) $(RELEASEDIR)/$(WINDIR64)
	cp $(LICENSE) $(RELEASEDIR)/$(DARWINDIR64)

	cp $(README) $(RELEASEDIR)/$(LINUXDIR64)
	cp $(README) $(RELEASEDIR)/$(WINDIR64)
	cp $(README) $(RELEASEDIR)/$(DARWINDIR64)

	cd $(SRCDIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build && mv $(EXE) ../$(RELEASEDIR)/$(LINUXDIR64)  
	cd $(SRCDIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build && mv $(EXE).exe ../$(RELEASEDIR)/$(WINDIR64)  
	cd $(SRCDIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build && mv $(EXE) ../$(RELEASEDIR)/$(DARWINDIR64)

	cd $(RELEASEDIR) && \
	zip -r $(LINUXDIR64) $(LINUXDIR64) && \
	zip -r $(WINDIR64) $(WINDIR64) && \
	zip -r $(DARWINDIR64) $(DARWINDIR64)

	mkdir -p $(RELEASEDIR)/$(LINUXDIR32)
	mkdir -p $(RELEASEDIR)/$(WINDIR32)
	mkdir -p $(RELEASEDIR)/$(DARWINDIR32)

	cp $(LICENSE) $(RELEASEDIR)/$(LINUXDIR32)
	cp $(LICENSE) $(RELEASEDIR)/$(WINDIR32)
	cp $(LICENSE) $(RELEASEDIR)/$(DARWINDIR32)

	cp $(README) $(RELEASEDIR)/$(LINUXDIR32)
	cp $(README) $(RELEASEDIR)/$(WINDIR32)
	cp $(README) $(RELEASEDIR)/$(DARWINDIR32)


	cd $(SRCDIR) && CGO_ENABLED=0 GOOS=linux GOARCH=386 go build && mv $(EXE) ../$(RELEASEDIR)/$(LINUXDIR32)  
	cd $(SRCDIR) && CGO_ENABLED=0 GOOS=windows GOARCH=386 go build && mv $(EXE).exe ../$(RELEASEDIR)/$(WINDIR32)  

	cd $(RELEASEDIR) && \
	zip -r $(LINUXDIR32) $(LINUXDIR32) && \
	zip -r $(WINDIR32) $(WINDIR32)

install: all
	mv $(EXE) /usr/local/bin/