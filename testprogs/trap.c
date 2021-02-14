#include <common.h>

static int trap_handler(word_t cause, word_t val, struct regs *regs)
{
	puts("HALTING");
	halt(0);
}

int main(void)
{
	set_trap_handler(trap_handler);
	ebreak();

	return 1;
}
