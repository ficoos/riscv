package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

const _ToolPrefix = "./toolchain/bin/riscv32-unknown-elf"
const _CC = _ToolPrefix + "-gcc"
const _AS = _ToolPrefix + "-as"
const _LD = _ToolPrefix + "-ld"
const _OBJCOPY = _ToolPrefix + "-objcopy"

type DebugBoard struct {
	board  *Board
	output *strings.Builder
}

func NewDebugBoard(prog []uint8) *DebugBoard {
	output := strings.Builder{}
	board := NewBoard(prog, nil, &output)
	return &DebugBoard{
		board:  board,
		output: &output,
	}
}

func (db *DebugBoard) Cpu() *Cpu {
	return db.board.Cpu()
}

type ProgArgs map[string]interface{}

type ProgTemplate struct {
	t  *template.Template
	sb *strings.Builder
}

func (pt *ProgTemplate) Execute(data interface{}) string {
	pt.sb.Reset()
	pt.t.Execute(pt.sb, data)
	return pt.sb.String()
}

func NewProgTemplate(tmpl string) *ProgTemplate {
	return &ProgTemplate{
		// we add the newline bause the assembler likes having a newline
		// at the end of the last command
		template.Must(template.New("prog").Parse(
			".global _start\n" +
				"_start:\n" +
				tmpl +
				"\n")),
		&strings.Builder{},
	}
}

func assertRegEq(t *testing.T, cpu *Cpu, reg uint8, v uint32) bool {
	regv := cpu.GetReg(reg)
	res := regv == v
	if !res {
		t.Errorf(("expected reg %d to have value 0x%08x" +
			" but it had value 0x%08x"), reg, v, regv)
	}

	return res
}

func assertCsrEq(t *testing.T, cpu *Cpu, csr uint32, v uint32) bool {
	regv := cpu.GetCsr(csr)
	res := regv == v
	if !res {
		t.Errorf(("expected csr 0x%03x to have value 0x%08x" +
			" but it had value 0x%08x"), csr, v, regv)
	}

	return res
}

func assertPcEq(t *testing.T, cpu *Cpu, v uint32) bool {
	res := cpu.pc == v
	if !res {
		t.Errorf(("expected pc to have value 0x%08x" +
			" but it had value 0x%08x"), v, cpu.pc)
	}

	return res
}

/*
func disassemble(prog []byte) string {
	cmd = exec.Command("riscv64-linux-gnu-objdump",
		"-m", "riscv:rv32",
		"-b", "binary",
		"-D",
		binPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("disassemble failed (", err, ") ", string(out)))
	}
	fmt.Println(string(out))
}
*/
func compile(prog string) []byte {
	dir, err := ioutil.TempDir("", "riscv_cpu_test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(dir)

	srcPath := filepath.Join(dir, "prog.c")
	objPath := filepath.Join(dir, "prog.o")
	elfPath := filepath.Join(dir, "prog.elf")
	binPath := filepath.Join(dir, "prog.bin")

	// write source
	err = ioutil.WriteFile(srcPath, []byte(prog), 0444)
	if err != nil {
		panic(err)
	}

	// compile
	cmd := exec.Command(_CC,
		"-c",
		"-Iruntime/",
		"-ffreestanding",
		"-nostdlib",
		"-std=c99",
		"-march=rv32i",
		"-mabi=ilp32",
		"-o", objPath,
		srcPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("compilation failed (", err, ") ", string(out)))
	}

	// link
	cmd = exec.Command(_LD,
		"-T", "runtime/prog.ld",
		"-static",
		"-m", "elf32lriscv",
		"-o", elfPath,
		objPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("linkage failed (", err, ") ", string(out)))
	}

	//@fixe: there is a bug in gnu-ld for riscv so we have to use objcopy
	// convert to binary
	cmd = exec.Command(_OBJCOPY,
		"-O", "binary",
		elfPath, binPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("copy failed (", err, ") ", string(out)))
	}

	// read binary
	res, err := ioutil.ReadFile(binPath)
	if err != nil {
		panic(err)
	}

	return res
}

func assemble(prog string) []byte {
	dir, err := ioutil.TempDir("", "riscv_cpu_test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(dir)

	srcPath := filepath.Join(dir, "prog.s")
	objPath := filepath.Join(dir, "prog.o")
	elfPath := filepath.Join(dir, "prog.elf")
	binPath := filepath.Join(dir, "prog.bin")

	// write source
	err = ioutil.WriteFile(srcPath, []byte(prog), 0444)
	if err != nil {
		panic(err)
	}

	// compile
	cmd := exec.Command(_AS,
		"-o", objPath,
		"-march=rv32i",
		"-mabi=ilp32",
		srcPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("compilation failed (", err, ") ", string(out)))
	}

	// link
	cmd = exec.Command(_LD,
		"-nostdlib",
		"-Ttext", "0x100",
		"-m", "elf32lriscv",
		"-o", elfPath, objPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("linkage failed (", err, ") ", string(out)))
	}

	// dump
	cmd = exec.Command(_OBJCOPY,
		"-O", "binary",
		elfPath, binPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprint("dump failed (", err, ") ", string(out)))
	}

	// read binary
	res, err := ioutil.ReadFile(binPath)
	if err != nil {
		panic(err)
	}

	return res
}

// since we can't check every permutation we check random permutations
// this defines how many random permutations to try
const FUZZ_ITER = 10

func randReg() uint8 {
	return uint8((rand.Uint32() % 31) + 1)
}

func sextend(imm int32) uint32 {
	vv := uint32(imm)
	if imm < 0 {
		vv |= 0xfffff800
	}
	return vv
}

func TestMOVI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		reg := randReg()
		imm := rand.Int31()%0xfff - 0x800
		prog := fmt.Sprintf("addi x%d, x0, %d", reg, imm)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.Step()
		expected := sextend(imm)
		if cpu.GetReg(reg) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(reg))
		}
	}
}

func TestADDI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		imm := rand.Int31()%0xfff - 0x800
		v := rand.Uint32()
		prog := fmt.Sprintf("addi x%d, x%d, %d", rd, rs1, imm)
		vv := sextend(imm)
		vv += v
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, v)
		cpu.Step()
		if cpu.GetReg(rd) != vv {
			t.Error("expected", vv, "got", cpu.GetReg(rd))
		}
	}
}

func TestSTLI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		v := rand.Int31()%0xfff - 0x800
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("slti x%d, x%d, %d", rd, rs1, v)
		var vv uint32
		if int32(rs1v) < v {
			vv = 1
		} else {
			vv = 0
		}
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != vv {
			t.Error("expected", vv, "got", cpu.GetReg(rd))
		}
	}
}

func TestSTLU(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		imm := rand.Int31()%0xfff - 0x800
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("sltu x%d, x%d, %d", rd, rs1, imm)
		var expected uint32
		if rs1v < sextend(imm) {
			expected = 1
		} else {
			expected = 0
		}
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestANDI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		imm := rand.Int31()%0xfff - 0x800
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("andi x%d, x%d, %d", rd, rs1, imm)
		expected := rs1v & sextend(imm)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestORI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		imm := rand.Int31()%0xfff - 0x800
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("ori x%d, x%d, %d", rd, rs1, imm)
		expected := rs1v | sextend(imm)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestXORI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		imm := rand.Int31()%0xfff - 0x800
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("xori x%d, x%d, %d", rd, rs1, imm)
		expected := rs1v ^ sextend(imm)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSLLI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		shamt := rand.Uint32() % 32
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("slli x%d, x%d, %d", rd, rs1, shamt)
		expected := rs1v << shamt
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSRLI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		shamt := rand.Uint32() % 32
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("srli x%d, x%d, %d", rd, rs1, shamt)
		expected := rs1v >> shamt
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSRAI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		shamt := rand.Uint32() % 32
		rs1v := rand.Uint32()
		prog := fmt.Sprintf("srai x%d, x%d, %d", rd, rs1, shamt)
		expected := uint32(int32(rs1v) >> shamt)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestLUI(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		imm := rand.Uint32() % 0xfffff
		prog := fmt.Sprintf("lui x%d, %d", rd, imm)
		expected := imm << 12
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestAUIPC(t *testing.T) {
	//@todo: this test assume pc is at 0
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		imm := rand.Uint32() % 0xfffff
		prog := fmt.Sprintf("auipc x%d, %d", rd, imm)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		expected := (imm << 12) + cpu.pc
		cpu.Step()
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestADD(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("add x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v + rs2v
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSUB(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("sub x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v - rs2v
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSLT(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("slt x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		var expected uint32
		if int32(rs1v) < int32(rs2v) {
			expected = 1
		} else {
			expected = 0
		}
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSLTU(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("sltu x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		var expected uint32
		if rs1v < rs2v {
			expected = 1
		} else {
			expected = 0
		}
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestAND(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("and x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v & rs2v
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestOR(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("or x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v | rs2v
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestXOR(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("xor x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v ^ rs2v
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSLL(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("sll x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v << (rs2v & 0x1f)
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSRL(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("srl x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := rs1v >> (rs2v & 0x1f)
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestSRA(t *testing.T) {
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := fmt.Sprintf("sra x%d, x%d, x%d", rd, rs1, rs2)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		expected := uint32(int32(rs1v) >> (rs2v & 0x1f))
		if cpu.GetReg(rd) != expected {
			t.Error("expected", expected, "got", cpu.GetReg(rd))
		}
	}
}

func TestJAL(t *testing.T) {
	progTmpl := NewProgTemplate(`jal x{{.rd}}, {{.offt}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		offt := (rand.Int31() % 0x3ffff) << 1
		prog := progTmpl.Execute(ProgArgs{"rd": rd, "offt": offt})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.Step()
		imm := uint32(offt)
		if offt < 0 {
			imm &= 0xfff00000
		}
		assertRegEq(t, cpu, rd, cpu.initialAddr+4)
		assertPcEq(t, cpu, imm)
	}
}

func TestJALR(t *testing.T) {
	//TODO: also test immediate offset
	progTmpl := NewProgTemplate(`jalr x{{.rd}}, x{{.rs1}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		rs1 := randReg()
		rs1v := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{"rd": rd, "rs1": rs1})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.Step()
		assertRegEq(t, cpu, rd, cpu.initialAddr+4)
		assertPcEq(t, cpu, rs1v&0xfffffffe)
	}
}

func testBranch(t *testing.T, opcode string, cond func(rs1v, rs2v uint32) bool) {
	progTmpl := NewProgTemplate(`
	{{.opcode}} x{{.rs1}}, x{{.rs2}}, true
	nop
	true:
	nop
	`)
	for i := 0; i < FUZZ_ITER; i++ {
		rs1 := randReg()
		rs2 := randReg()
		rs1v := rand.Uint32()
		rs2v := rand.Uint32()
		if rs1 == rs2 {
			rs2v = rs1v
		}
		prog := progTmpl.Execute(ProgArgs{
			"opcode": opcode,
			"rs1":    rs1,
			"rs2":    rs2,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, rs1v)
		cpu.SetReg(rs2, rs2v)
		cpu.Step()
		var expected uint32
		if cond(rs1v, rs2v) {
			expected = cpu.initialAddr + 8
		} else {
			expected = cpu.initialAddr + 4
		}
		assertPcEq(t, cpu, expected)
	}
}

func TestBEQ(t *testing.T) {
	testBranch(t, "beq", func(rs1v, rs2v uint32) bool { return rs1v == rs2v })
}

func TestBNE(t *testing.T) {
	testBranch(t, "bne", func(rs1v, rs2v uint32) bool { return rs1v != rs2v })
}

func TestBLT(t *testing.T) {
	testBranch(t, "blt", func(rs1v, rs2v uint32) bool { return int32(rs1v) < int32(rs2v) })
}

func TestBLTU(t *testing.T) {
	testBranch(t, "bltu", func(rs1v, rs2v uint32) bool { return rs1v < rs2v })
}

func TestBGE(t *testing.T) {
	testBranch(t, "bge", func(rs1v, rs2v uint32) bool { return int32(rs1v) > int32(rs2v) })
}

func TestBGEU(t *testing.T) {
	testBranch(t, "bgeu", func(rs1v, rs2v uint32) bool { return rs1v > rs2v })
}

func testLd(t *testing.T, opcode string, bits uint, isSigned bool) {
	progTmpl := NewProgTemplate(`
	{{.opcode}} x{{.rd}}, data
	data:
	.word {{.v}}
	`)
	for i := 0; i < FUZZ_ITER; i++ {
		dest := randReg()
		v := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"opcode": opcode,
			"rd":     dest,
			"v":      v,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		//@investigate: load instruction get broken down to 2
		// instructions by the assmebler I need to figure out
		// how to make sure only one is made is that this is
		// a unit test for single instruction like all the others
		cpu.Step()
		cpu.Step()
		expected := v & ((1 << bits) - 1)
		if isSigned {
			expected = signExtend(expected, bits)
		}
		assertRegEq(t, cpu, dest, expected)
	}
}

func TestLW(t *testing.T) {
	testLd(t, "lw", 32, false)
}

func TestLH(t *testing.T) {
	testLd(t, "lh", 16, true)
}

func TestLB(t *testing.T) {
	testLd(t, "lb", 8, true)
}

func TestLBU(t *testing.T) {
	testLd(t, "lbu", 8, false)
}

func TestLHU(t *testing.T) {
	testLd(t, "lhu", 16, false)
}

func testStr(t *testing.T, opcode string, bits uint) {
	progTmpl := NewProgTemplate(`
	{{.opcode}} x{{.rs1}}, 0x104(x0)
	data:
	.word 0
	`)
	for i := 0; i < FUZZ_ITER; i++ {
		rs1 := randReg()
		v := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"opcode": opcode,
			"rs1":    rs1,
			"v":      v,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rs1, v)
		cpu.Step()
		cpu.Step()
		expected := v & ((1 << bits) - 1)
		if cpu.memory.LoadWord(cpu.initialAddr+4) != expected {
			t.Errorf("expected 0x%x got 0x%x", expected, cpu.LoadWord(4))
		}
	}
}

func TestSW(t *testing.T) {
	testStr(t, "sw", 32)
}

func TestSH(t *testing.T) {
	testStr(t, "sh", 16)
}

func TestSB(t *testing.T) {
	testStr(t, "sb", 8)
}

func TestRdcycle(t *testing.T) {
	progTmpl := NewProgTemplate(`
		rdcycle x{{.rd}}
		`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		cycles := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.cycles = cycles
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(cycles))
	}
}

func TestRdcycleh(t *testing.T) {
	progTmpl := NewProgTemplate(`
		rdcycleh x{{.rd}}
		`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		cycles := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.cycles = cycles
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(cycles>>32))
	}
}

func TestRdtime(t *testing.T) {
	progTmpl := NewProgTemplate(`
		rdtime x{{.rd}}
		`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		ticks := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.ticks = ticks
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(ticks))
	}
}

func TestRdtimeh(t *testing.T) {
	progTmpl := NewProgTemplate(`
		rdtimeh x{{.rd}}
		`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		ticks := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.ticks = ticks
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(ticks>>32))
	}
}

func TestRdinstret(t *testing.T) {
	progTmpl := NewProgTemplate(`rdinstret x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		instret := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.instret = instret
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(instret))
	}
}

func TestRdinstreth(t *testing.T) {
	progTmpl := NewProgTemplate(`rdinstreth x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		instret := rand.Uint64()
		prog := progTmpl.Execute(ProgArgs{"rd": rd})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.instret = instret
		cpu.Step()
		assertRegEq(t, cpu, rd, uint32(instret>>32))
	}
}

func TestReadCsr(t *testing.T) {
	progTmpl := NewProgTemplate(`csrrw x{{.rd}}, mscratch, x0`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		v := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"rd": rd,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetCsr(CsrScratch|CsrM, v)
		cpu.Step()
		assertRegEq(t, cpu, rd, v)
		assertCsrEq(t, cpu, CsrScratch|CsrM, v)
	}
}

func TestWriteCsr(t *testing.T) {
	progTmpl := NewProgTemplate(`csrrw x0, mscratch, x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := randReg()
		v := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"rd": rd,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rd, v)
		cpu.Step()
		assertCsrEq(t, cpu, CsrScratch|CsrM, v)
	}
}

func TestCsrrw(t *testing.T) {
	progTmpl := NewProgTemplate(`csrrw x{{.rd}}, mscratch, x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := (randReg() % 30) + 1
		csrv := rand.Uint32()
		rdv := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"rd": rd,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rd, rdv)
		cpu.SetCsr(CsrScratch|CsrM, csrv)
		cpu.Step()
		assertCsrEq(t, cpu, CsrScratch|CsrM, rdv)
		assertRegEq(t, cpu, rd, csrv)
	}
}

func TestCsrrs(t *testing.T) {
	progTmpl := NewProgTemplate(`csrrs x{{.rd}}, mscratch, x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := (randReg() % 30) + 1
		csrv := rand.Uint32()
		rdv := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"rd": rd,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rd, rdv)
		cpu.SetCsr(CsrScratch|CsrM, csrv)
		cpu.Step()
		assertCsrEq(t, cpu, CsrScratch|CsrM, csrv&rdv)
		assertRegEq(t, cpu, rd, csrv)
	}
}

func TestCsrrc(t *testing.T) {
	progTmpl := NewProgTemplate(`csrrc x{{.rd}}, mscratch, x{{.rd}}`)
	for i := 0; i < FUZZ_ITER; i++ {
		rd := (randReg() % 30) + 1
		csrv := rand.Uint32()
		rdv := rand.Uint32()
		prog := progTmpl.Execute(ProgArgs{
			"rd": rd,
		})
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.SetReg(rd, rdv)
		cpu.SetCsr(CsrScratch|CsrM, csrv)
		cpu.Step()
		assertCsrEq(t, cpu, CsrScratch|CsrM, csrv&(^rdv))
		assertRegEq(t, cpu, rd, csrv)
	}
}

func TestEbreak(t *testing.T) {
	progTmpl := NewProgTemplate(`ebreak`)
	for i := 0; i < FUZZ_ITER; i++ {
		prog := progTmpl.Execute(nil)
		t.Log("prog: ", prog)
		cpu := NewDebugBoard(assemble(prog)).Cpu()
		cpu.Step()
		assertCsrEq(t, cpu, CsrEpc|CsrM, cpu.initialAddr)
		assertCsrEq(t, cpu, CsrTval|CsrM, cpu.initialAddr)
		assertCsrEq(t, cpu, CsrCause|CsrM, ExceptionBreakpoint)
	}
}

func TestProgs(t *testing.T) {
	files, err := ioutil.ReadDir("./testprogs")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".c") {
			continue
		}
		t.Log("Running test prog:", f.Name())
		prog, err := ioutil.ReadFile("./testprogs/" + f.Name())
		if err != nil {
			panic(err)
		}
		board := NewDebugBoard(compile(string(prog)))
		cpu := board.Cpu()
		cpu.Execute()
		output := strings.TrimSpace(board.output.String())
		if output != "" {
			t.Log("output: ", output)
		}
		if cpu.GetCsr(CsrHalt) != 0 {
			t.Errorf("Execution failed")
		}

		outfile := "./testprogs/" + f.Name() + ".out"
		if _, err := os.Stat(outfile); err == nil {
			expected, err := ioutil.ReadFile(outfile)
			expectedOutput := strings.TrimSpace(string(expected))
			if err != nil {
				t.Error("Failed to read output file:", err)
			}

			if strings.Compare(expectedOutput, output) != 0 {
				t.Errorf("Expected output:\n%s\nGot output:\n%s\n", expectedOutput, output)
			}
		}
	}

}
