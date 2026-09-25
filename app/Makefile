# The Dockerfile's Go stage copies the binary in; acap-build always runs make,
# so these targets only satisfy it.
PROG = Tailcat

.PHONY: all clean

all: $(PROG)

$(PROG):
	@test -x $@ || { echo "$@ missing: the Go build stage should have copied it in" >&2; exit 1; }

clean:
	@:
