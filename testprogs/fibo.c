#define N 50

void putc(int c) {
}

int main(void)
{
	int a = 1;
	int b = 1;
	for (int i = 2; i < N; i++) {
		int tmp = a + b;
		a = b;
		b = tmp;
	}

	return 0;
}
