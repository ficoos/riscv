.equ mhalt, 0x3ff
.text
# Trap handling

# For ease of use all traps will allways use the trampoline
# that will do the tricky things to back up and restore state.
# The trampolin will pass the state to a handler that can modify the state
# before returning

.global trap_trampolin
trap_trampolin:
# Back up sp and replace with a clean valid stack.
# The trap might have been cause by sp being misaligned or due to a stack
# overflow, this means we might not be able to use this stack
csrrw zero, mscratch, sp
la sp, kstack_start
# store registers in the stack
addi sp, sp, -128
sw x1, (sp)
# skip sp, we backed it up in mscratch and we'll put it in the stack later
sw x3, 8(sp)
sw x4, 12(sp)
sw x5, 16(sp)
sw x6, 20(sp)
sw x7, 24(sp)
sw x8, 28(sp)
sw x9, 32(sp)
sw x10, 36(sp)
sw x11, 40(sp)
sw x12, 44(sp)
sw x13, 48(sp)
sw x14, 52(sp)
sw x15, 56(sp)
sw x16, 60(sp)
sw x17, 64(sp)
sw x18, 68(sp)
sw x19, 72(sp)
sw x20, 76(sp)
sw x21, 80(sp)
sw x22, 84(sp)
sw x23, 88(sp)
sw x24, 92(sp)
sw x25, 96(sp)
sw x26, 100(sp)
sw x27, 104(sp)
sw x28, 108(sp)
sw x29, 112(sp)
sw x30, 116(sp)
sw x31, 120(sp)
# now that we have all the register backed up we can use them
# put sp in the register vector
csrrw t0, mscratch, zero
sw t0, 4(sp)
# ip is also a register the trap handler might want to adjust
csrrw t0, mepc, zero
sw t0, 124(sp)

# call trap handler
csrrw a0, mcause, zero
csrrw a1, mtval, zero
mv a2, sp # the registers
la t0, trap_handler
lw t0, (t0)
jalr t0

# restore state
# set ret pc
lw t0, 124(sp)
csrrw zero, mepc, t0
# keep sp in mscratch
lw t0, 4(sp)
csrrw zero, mscratch, t0
# load registers from the stack
lw x1, (sp)
# skip sp, we still need the current stack
lw x3, 8(sp)
lw x4, 12(sp)
lw x5, 16(sp)
lw x6, 20(sp)
lw x7, 24(sp)
lw x8, 28(sp)
lw x9, 32(sp)
lw x10, 36(sp)
lw x11, 40(sp)
lw x12, 44(sp)
lw x13, 48(sp)
lw x14, 52(sp)
lw x15, 56(sp)
lw x16, 60(sp)
lw x17, 64(sp)
lw x18, 68(sp)
lw x19, 72(sp)
lw x20, 76(sp)
lw x21, 80(sp)
lw x22, 84(sp)
lw x23, 88(sp)
lw x24, 92(sp)
lw x25, 96(sp)
lw x26, 100(sp)
lw x27, 104(sp)
lw x28, 108(sp)
lw x29, 112(sp)
lw x30, 116(sp)
lw x31, 120(sp)
# go back to the users sp
csrrw sp, mscratch, zero
# return back to user code
mret

trap_handler:
.word default_trap_handler

.global set_trap_handler
set_trap_handler:
la t0, trap_handler
sw a0, (t0)

.global ebreak
ebreak:
ebreak

.global halt
halt:
csrrw x0, mhalt, a0

kstack_end:
.fill 4096, 1, 0
kstack_start:
.word 0xfefefefe

