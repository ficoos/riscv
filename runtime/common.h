#ifndef TT_LIBCOMMON_H
#define TT_LIBCOMMON_H

#define NULL ((void*)0)
#define NGPR 31

typedef long size_t;
typedef unsigned long word_t;
typedef word_t uintptr_t;

struct regs {
	union {
		struct {
			word_t x1;
			word_t x2;
			word_t x3;
			word_t x4;
			word_t x5;
			word_t x6;
			word_t x7;
			word_t x8;
			word_t x9;
			word_t x10;
			word_t x11;
			word_t x12;
			word_t x13;
			word_t x14;
			word_t x15;
			word_t x16;
			word_t x17;
			word_t x18;
			word_t x19;
			word_t x20;
			word_t x21;
			word_t x22;
			word_t x23;
			word_t x24;
			word_t x25;
			word_t x26;
			word_t x27;
			word_t x28;
			word_t x29;
			word_t x30;
			word_t x31;
		};
		// mnemonics
		struct {
			word_t ra;
			word_t sp;
			word_t gp;
			word_t tp;
			word_t t0;
			word_t t1;
			word_t t2;
			word_t s0;
			word_t s1;
			word_t a0;
			word_t a1;
			word_t a2;
			word_t a3;
			word_t a4;
			word_t a5;
			word_t a6;
			word_t a7;
			word_t s2;
			word_t s3;
			word_t s4;
			word_t s5;
			word_t s6;
			word_t s7;
			word_t s8;
			word_t s9;
			word_t s10;
			word_t s11;
			word_t t3;
			word_t t4;
			word_t t5;
			word_t t6;
		};
		word_t gpr[NGPR];
	};
	word_t pc;
} __attribute__((packed));
typedef void (*trap_handler_t)(word_t cause, word_t val, struct regs *regs);
void set_trap_handler(trap_handler_t trap_handler);
void ebreak(void);

int putchar(int c);
int getchar(void);
int puts(const char *s);
char *ltoa(long value, char *str, int base);
void halt(word_t rv);
void dumpregs(struct regs *regs);
static inline char *itoa(int value, char *str, int base)
{
	return ltoa((long)value, str, base);
}

#endif // TT_LIBCOMMON_H
