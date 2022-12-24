SRCDIR:=src
EXE:=websurf
TESTDIR:=testing

all:
	cd $(SRCDIR) && go build && mv $(EXE) ..

test: all
	rm -rf $(TESTDIR) && \
	mkdir -p $(TESTDIR) && \
	cp $(EXE) $(TESTDIR) && \
	cp conf.json $(TESTDIR)

clean:
	rm -rf $(TESTDIR) $(EXE)