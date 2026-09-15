# The application binary is cross-compiled by the Go stage in the Dockerfile and
# copied into this directory before packaging, so there is nothing to compile
# here. acap-build always invokes make, so these targets exist to satisfy it.
PROG = Tailcat

.PHONY: all clean

all: $(PROG)

$(PROG):
	@test -x $@ || { echo "$@ missing: the Go build stage should have copied it in" >&2; exit 1; }

clean:
	@:
