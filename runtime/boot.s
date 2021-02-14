.section .bootstrap
.global _start
_start:

# set up trap handling
la t0, trap_trampolin
csrrw zero, mtvec, t0

# set up the stack
la sp, stack_start

# start main
jal main
end:
jal halt
j end
# make sure we don't just slide

stack_end:
.fill 4096, 1, 0
stack_start:
