TOOL_PREFIX?=riscv64-linux-gnu
AS?=${TOOL_PREFIX}-as
CC?=${TOOL_PREFIX}-gcc
AR?=${TOOL_PREFIX}-ar

ARCH?=rv32i
ABI?=ilp32

TARGET=rt.a
headers=common.h
objs=boot.o common.o trap.o

LIBGCC_LOCATION=$(shell ${CC} -print-libgcc-file-name)

.PHONY: all
all: ${TARGET} libgcc.a

libgcc.a: ${LIBGCC_LOCATION}
	cp $< $@
%.o: %.s ${headers}
	${AS} -march=${ARCH} -mabi=${ABI} -c -o $@ $<

%.o: %.c ${headers}
	${CC} -march=${ARCH} -mabi=${ABI} -c -o $@ $<

${TARGET}: ${objs}
	${AR} cr -o $@ $^

.PHONY: clean
clean:
	${RM} ${TARGET} ${objs}
