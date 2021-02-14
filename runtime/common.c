#include "common.h"

static volatile char *IO_ADDR = (char*)0xfffffffe;

// We have this to avoid internally paying for the function
// call
static inline void inline_putchar(int c) {
	*IO_ADDR = c;
}

int putchar(int c)
{
	inline_putchar(c);
	return c;
}

int getchar(void)
{
	return *IO_ADDR;
}

int puts(const char *s)
{
	for (const char *c = s; *c != '\0'; c++) {
		inline_putchar(*c);
	}

	return 1;
}

static const char hex_chars[] = "0123456789ABCDEF";
char *ltoa(long value, char *str, int base)
{
	if (base < 2 || base > 16) {
		return NULL;
	}

	int i = 0;
	do {
		str[i] = hex_chars[value % base];
		value /= base;
		i++;
	} while (value > 0);

	str[i] = '\0';

	/* reverse */
	char *start = str;
	char *end = start + i - 1;
	char temp;

	while (end > start) {
		temp = *start;
		*start = *end;
		*end = temp;

		++start;
		--end;
	}

	return str;
}

static char *ltohex(unsigned long v, char *s) {
	for (int i = 7; i >= 0; i--) {
		s[i] = hex_chars[(v & 0xf)];
		v >>= 4;
	}
	s[8] = '\0';

	return s;
}

static const char *strtrap(word_t cause) {
	switch (cause) {
		case 11:
			return "ecall";
		case 3:
			return "ebreak";
		case 2:
			return "illegal instruction";
		default:
			return "unknown trap";
	}
}

void dumpregs(struct regs *regs)
{
	char tmp[9];
	for (int i = 0; i < NGPR; i++) {
		if (i < 9) {
			putchar(' ');
		}
		putchar('x');
		puts(ltoa(i + 1, tmp, 10));
		puts(": 0x");
		puts(ltohex(regs->gpr[i], tmp));
		putchar(i % 4 == 3 ? '\n' : ' ');
	}
	puts(" pc: 0x"); puts(ltohex(regs->pc, tmp));
	puts("\n");
}

void default_trap_handler(word_t cause, word_t val, struct regs *regs)
{
	char tmp[256];
	puts("\nIT'S A TRAP!\n");
	puts("cause: "); puts(strtrap(cause));
	puts(" ("); puts(ltoa(cause, tmp, 10)); puts(") ");
	puts(" val: 0x"); puts(ltohex(val, tmp));
	putchar('\n');
	dumpregs(regs);
	//@todo: figure out a convention to make trap halts be different than
	//       normal error halts
	halt(cause);
}
