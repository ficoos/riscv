package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

const (
	OP_IMM    = 0x13
	OP_LUI    = 0x37
	OP_AUIPC  = 0x17
	OP        = 0x33
	OP_JAL    = 0x6f
	OP_JALR   = 0x67
	OP_BRANCH = 0x63
	OP_LOAD   = 0x03
	OP_STORE  = 0x23
	OP_SYSTEM = 0x73
)

// OP_IMM
const (
	FUNCT_ADDI  = 0
	FUNCT_SLLI  = 1
	FUNCT_SLTI  = 2
	FUNCT_SLTUI = 3
	FUNCT_XORI  = 4
	FUNCT_SRXI  = 5
	FUNCT_ORI   = 6
	FUNCT_ANDI  = 7
)

// OP
const (
	FUNCT_ADD_SUB = 0
	FUNCT_SLL     = 1
	FUNCT_SLT     = 2
	FUNCT_SLTU    = 3
	FUNCT_XOR     = 4
	FUNCT_SRX     = 5
	FUNCT_OR      = 6
	FUNCT_AND     = 7
)

// BRANCH
const (
	FUNCT_BEQ  = 0
	FUNCT_BNE  = 1
	FUNCT_BLT  = 4
	FUNCT_BGE  = 5
	FUNCT_BLTU = 6
	FUNCT_BGEU = 7
)

// SYSTEM
const (
	FUNCT_CSRRW = 1
	FUNCT_CSRRS = 2
	FUNCT_CSRRC = 3
	FUNCT_PRIV  = 0
)

// SYSTEM PRIV
const (
	PRIV_EBREAK = 0x1
	PRIV_ECALL  = 0x00
)

// CSRs
const (
	CsrM = 0x300
	CsrS = 0x100
	CsrU = 0x000

	CsrStatus   = 0x000
	CsrIe       = 0x004
	CsrTvec     = 0x005
	CsrScratch  = 0x040
	CsrEpc      = 0x041
	CsrCause    = 0x042
	CsrTval     = 0x043
	CsrCycle    = 0xc00
	CsrCycleh   = 0xc80
	CsrTime     = 0xc01
	CsrTimeh    = 0xc81
	CsrInstret  = 0xc02
	CsrInstreth = 0xc82
	CsrHalt     = 0x3ff
)

// Exceptions
const (
	ExceptionIllegalInstruction = 2
	ExceptionBreakpoint         = 3
	ExceptionEcallM             = 11
)

const (
	RegZero = 0
	RegRA   = 1
	RegSP   = 2
	RegGP   = 3
	RegTP   = 4
	RegT0   = 5
	RegT1   = 6
	RegT2   = 7
	RegFP   = 8
	RegS0   = 8
	RegS1   = 9
	RegA0   = 10
	RegA1   = 11
	RegA2   = 12
	RegA3   = 13
	RegA4   = 14
	RegA5   = 15
	RegA6   = 16
	RegA7   = 17
	RegS2   = 18
	RegS3   = 19
	RegS4   = 20
	RegS5   = 21
	RegS6   = 22
	RegS7   = 23
	RegS8   = 24
	RegS9   = 25
	RegS10  = 26
	RegS11  = 27
	RegST3  = 28
	RegST4  = 29
	RegST5  = 30
	RegST6  = 31
)

var _RegNames []string = []string{
	"zero",
	"ra",
	"sp",
	"gp",
	"tp",
	"t0",
	"t1",
	"t2",
	"FP",
	"S0",
	"S1",
	"A0",
	"A1",
	"A2",
	"A3",
	"A4",
	"A5",
	"A6",
	"A7",
	"S2",
	"S3",
	"S4",
	"S5",
	"S6",
	"S7",
	"S8",
	"S9",
	"S10",
	"S11",
	"ST3",
	"ST4",
	"ST5",
	"ST6",
}

type Memory interface {
	LoadWord(addr uint32) uint32
	LoadHalfWord(addr uint32) uint16
	LoadByte(addr uint32) uint8
	StoreWord(addr uint32, v uint32)
	StoreHalfWord(addr uint32, v uint16)
	StoreByte(addr uint32, v uint8)
}

type Ram struct {
	memory []uint8
}

func NewRam(size uint32) *Ram {
	return NewRamFromBuffer(make([]uint8, size))
}

func NewRamFromBuffer(buf []uint8) *Ram {
	return &Ram{buf}
}

func (mem *Ram) LoadWord(addr uint32) uint32 {
	return binary.LittleEndian.Uint32(mem.memory[addr : addr+4])
}

func (mem *Ram) LoadHalfWord(addr uint32) uint16 {
	return binary.LittleEndian.Uint16(mem.memory[addr : addr+2])
}

func (mem *Ram) LoadByte(addr uint32) uint8 {
	return mem.memory[addr]
}

func (mem *Ram) StoreWord(addr uint32, v uint32) {
	binary.LittleEndian.PutUint32(mem.memory[addr:addr+4], v)
}

func (mem *Ram) StoreHalfWord(addr uint32, v uint16) {
	binary.LittleEndian.PutUint16(mem.memory[addr:addr+2], v)
}

func (mem *Ram) StoreByte(addr uint32, v uint8) {
	mem.memory[addr] = v
}

type Range struct {
	Addr, Size uint32
	Memory     Memory
}

type Mmu struct {
	ranges []Range
}

func NewMmu() *Mmu {
	return &Mmu{}
}

func (mmu *Mmu) AddRange(addr, size uint32, mem Memory) {
	//@todo: sanity checks
	mmu.ranges = append(mmu.ranges, Range{addr, size, mem})
}

func (mmu *Mmu) findRange(addr uint32) (*Range, uint32) {
	for _, r := range mmu.ranges {
		if addr >= r.Addr && addr < (r.Addr+r.Size) {
			return &r, addr - r.Addr
		}
	}
	return nil, 0
}

func (mmu *Mmu) LoadWord(addr uint32) uint32 {
	r, addr := mmu.findRange(addr)
	if r != nil {
		return r.Memory.LoadWord(addr)
	}

	return 0
}

func (mmu *Mmu) LoadHalfWord(addr uint32) uint16 {
	r, addr := mmu.findRange(addr)
	if r != nil {
		return r.Memory.LoadHalfWord(addr)
	}

	return 0
}

func (mmu *Mmu) LoadByte(addr uint32) uint8 {
	r, addr := mmu.findRange(addr)
	if r != nil {
		return r.Memory.LoadByte(addr)
	}

	return 0
}

func (mmu *Mmu) StoreWord(addr uint32, v uint32) {
	r, addr := mmu.findRange(addr)
	if r != nil {
		r.Memory.StoreWord(addr, v)
	}
}

func (mmu *Mmu) StoreHalfWord(addr uint32, v uint16) {
	r, addr := mmu.findRange(addr)
	if r != nil {
		r.Memory.StoreHalfWord(addr, v)
	}
}

func (mmu *Mmu) StoreByte(addr uint32, v uint8) {
	r, addr := mmu.findRange(addr)
	if r != nil {
		r.Memory.StoreByte(addr, v)
	}
}

type MmioSerial struct {
	w io.Writer
	r io.Reader
}

func (s *MmioSerial) LoadWord(addr uint32) uint32 {
	return uint32(s.LoadByte(addr))
}

func (s *MmioSerial) LoadHalfWord(addr uint32) uint16 {
	return uint16(s.LoadByte(addr))
}

func (s *MmioSerial) LoadByte(addr uint32) uint8 {
	if s.r == nil {
		return 0
	}

	var b [1]uint8
	s.r.Read(b[:])

	return b[0]
}

func (s *MmioSerial) StoreWord(addr uint32, v uint32) {
	s.StoreByte(addr, uint8(v))
}

func (s *MmioSerial) StoreHalfWord(addr uint32, v uint16) {
	s.StoreByte(addr, uint8(v))
}

func (s *MmioSerial) StoreByte(addr uint32, v uint8) {
	if s.w == nil {
		return
	}
	b := []uint8{v}
	s.w.Write(b)
}

type Cpu struct {
	initialAddr uint32
	registers   [32]uint32
	pc          uint32
	memory      Memory
	halt        bool
	cycles      uint64
	ticks       uint64
	instret     uint64
	mtvec       uint32
	mcause      uint32
	mepc        uint32
	mtval       uint32
	mscratch    uint32
	haltValue   uint32
}

func New(memory Memory, initialAddr uint32) *Cpu {
	cpu := &Cpu{}
	cpu.initialAddr = initialAddr
	cpu.memory = memory
	cpu.Reset()
	return cpu
}

func (cpu *Cpu) LoadWord(addr uint32) uint32 {
	return cpu.memory.LoadWord(addr)
}
func (cpu *Cpu) LoadHalfWord(addr uint32) uint16 {
	return cpu.memory.LoadHalfWord(addr)
}
func (cpu *Cpu) LoadByte(addr uint32) uint8 {
	return cpu.memory.LoadByte(addr)
}
func (cpu *Cpu) StoreWord(addr uint32, v uint32) {
	cpu.memory.StoreWord(addr, v)
}
func (cpu *Cpu) StoreHalfWord(addr uint32, v uint16) {
	cpu.memory.StoreHalfWord(addr, v)
}
func (cpu *Cpu) StoreByte(addr uint32, v uint8) {
	cpu.memory.StoreByte(addr, v)
}

func (cpu *Cpu) IsValidCsr(csr uint32) bool {
	if csr == CsrHalt {
		return true
	}
	priv := csr & ^uint32(0xcff) // save priv
	csr &= 0xcff                 // ignore priv
	switch csr {
	case CsrCycle,
		CsrCycleh,
		CsrTime,
		CsrTimeh,
		CsrInstret,
		CsrInstreth:

		return true
	}
	if priv != CsrM {
		return false
	}
	switch csr {
	case CsrTvec,
		CsrTval,
		CsrCause,
		CsrEpc,
		CsrScratch:

		return true
	}
	return false
}

func (cpu *Cpu) GetCsr(csr uint32) uint32 {
	if csr == CsrHalt {
		return cpu.haltValue
	}
	priv := csr & ^uint32(0xcff) // save priv
	csr &= 0xcff                 // ignore priv
	switch csr {
	case CsrCycle:
		return uint32(cpu.cycles)
	case CsrCycleh:
		return uint32(cpu.cycles >> 32)
	case CsrTime:
		return uint32(cpu.ticks)
	case CsrTimeh:
		return uint32(cpu.ticks >> 32)
	case CsrInstret:
		return uint32(cpu.instret)
	case CsrInstreth:
		return uint32(cpu.instret >> 32)
	}

	// we only have machine mode csrs for everything else
	if priv != CsrM {
		panic(fmt.Sprintf("invalid csr: 0x%03x\n", csr))
	}

	switch csr {
	case CsrTvec:
		return cpu.mtvec & 0xfffffffc
	case CsrTval:
		return cpu.mtval
	case CsrCause:
		return cpu.mcause
	case CsrEpc:
		return cpu.mepc & 0xfffffffe
	case CsrScratch:
		return cpu.mscratch
	default:
		fmt.Printf("invalid csr: 0x%03x\n", csr)
	}
	return 0
}

func (cpu *Cpu) SetCsr(csr uint32, v uint32) {
	if csr == CsrHalt {
		cpu.halt = true
		cpu.haltValue = v
		return
	}
	priv := csr & ^uint32(0xcff) // save priv
	if priv != CsrM {
		panic(fmt.Sprintf("invalid csr: 0x%03x\n", csr))
	}
	csr &= 0xcff // ignore priv
	switch csr {
	case CsrTvec:
		cpu.mtvec = v & 0xfffffffc
	case CsrCause:
		cpu.mcause = v
	case CsrTval:
		cpu.mtval = v
	case CsrScratch:
		cpu.mscratch = v
	case CsrEpc:
		cpu.mepc = v & 0xfffffffe
	}
	// do nothing
}

func (cpu *Cpu) Reset() {
	for i, _ := range cpu.registers {
		cpu.registers[i] = 0
	}
	cpu.pc = cpu.initialAddr
	cpu.halt = false
	cpu.cycles = 0
	cpu.ticks = 0
	cpu.instret = 0
	cpu.mtvec = 0
	cpu.mcause = 0
	cpu.mepc = 0
	cpu.mtval = 0
	cpu.mscratch = 0
}

func (cpu *Cpu) GetReg(idx uint8) uint32 {
	if idx == 0 {
		return 0
	} else if idx > 0 && idx < 32 {
		return cpu.registers[idx]
	}

	panic(fmt.Sprint("invalid register ", idx))
}

func (cpu *Cpu) SetReg(idx uint8, v uint32) {
	if idx == 0 {
		// do nothing
	} else if idx > 0 && idx < 32 {
		cpu.registers[idx] = v
	} else {
		panic(fmt.Sprint("invalid register ", idx))
	}
}

func (cpu *Cpu) Execute() {
	for !cpu.halt {
		cpu.Step()
	}
}

func (cpu *Cpu) Halt() {
	cpu.halt = true
}

func (cpu *Cpu) Debug() string {
	res := ""
	for i := uint8(1); i < 32; i++ {
		res += fmt.Sprintf("%s: 0x%08x ", _RegNames[i], cpu.GetReg(i))
	}
	res += fmt.Sprintf("pc: 0x%08x ", cpu.pc)
	return res
}

func (cpu *Cpu) fetch() uint32 {
	inst := cpu.LoadWord(cpu.pc)
	cpu.pc += 4

	return inst
}

func (cpu *Cpu) decode(inst uint32) {
	// we are only allowed to trap in the decode phase
	// this makes it so the trap function is only visible here
	trap := func(cause uint32, value uint32) {
		cpu.SetCsr(CsrTval|CsrM, value)
		cpu.SetCsr(CsrEpc|CsrM, cpu.pc-4)
		cpu.pc = cpu.GetCsr(CsrTvec | CsrM)
		cpu.SetCsr(CsrCause|CsrM, cause)
		cpu.cycles += 1
		cpu.ticks += 1
	}
	opcode := inst & 0x7f
decode:
	switch opcode {
	case OP_IMM:
		_, rd, funct, rs1, imm := itype(inst)
		rs1v := cpu.GetReg(rs1)
		var res uint32
		switch funct {
		case FUNCT_ADDI:
			res = rs1v + imm
		case FUNCT_SLTI:
			if int32(rs1v) < int32(imm) {
				res = 1
			} else {
				res = 0
			}
		case FUNCT_SLTUI:
			if rs1v < imm {
				res = 1
			} else {
				res = 0
			}
		case FUNCT_XORI:
			res = rs1v ^ imm
		case FUNCT_ANDI:
			res = rs1v & imm
		case FUNCT_ORI:
			res = rs1v | imm
		case FUNCT_SLLI:
			res = rs1v << imm
		case FUNCT_SRXI:
			if imm&0x400 != 0 {
				// golang does arithmatic shift for ints
				res = uint32(int32(rs1v) >> (imm & 0x1f))
			} else {
				res = rs1v >> (imm & 0x1f)
			}
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}
		cpu.SetReg(rd, res)
	case OP_LUI:
		_, rd, imm := utype(inst)
		cpu.SetReg(rd, imm<<12)
	case OP_AUIPC:
		_, rd, imm := utype(inst)
		cpu.SetReg(rd, cpu.pc+(imm<<12)-4)
	case OP:
		_, rd, funct3, rs1, rs2, funct7 := rtype(inst)
		rs1v := cpu.GetReg(rs1)
		rs2v := cpu.GetReg(rs2)
		var res uint32
		switch funct3 {
		case FUNCT_ADD_SUB:
			if funct7&0x20 == 0 {
				res = rs1v + rs2v
			} else {
				res = rs1v - rs2v
			}
		case FUNCT_SLT:
			if int32(rs1v) < int32(rs2v) {
				res = 1
			} else {
				res = 0
			}
		case FUNCT_SLTU:
			if rs1v < rs2v {
				res = 1
			} else {
				res = 0
			}
		case FUNCT_AND:
			res = rs1v & rs2v
		case FUNCT_OR:
			res = rs1v | rs2v
		case FUNCT_XOR:
			res = rs1v ^ rs2v
		case FUNCT_SLL:
			res = rs1v << (rs2v & 0x1f)
		case FUNCT_SRX:
			if funct7&0x20 == 0 {
				res = rs1v >> (rs2v & 0x1f)
			} else {
				res = uint32(int32(rs1v) >> (rs2v & 0x1f))
			}
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}
		cpu.SetReg(rd, res)
	case OP_JAL:
		_, rd, imm := jtype(inst)
		cpu.SetReg(rd, cpu.pc)
		cpu.pc += imm - 4
	case OP_JALR:
		_, rd, _, rs1, imm := itype(inst)
		rs1v := cpu.GetReg(rs1)
		cpu.SetReg(rd, cpu.pc)
		cpu.pc = (rs1v + imm) & 0xfffffffe
	case OP_BRANCH:
		_, funct3, rs1, rs2, imm := btype(inst)
		rs1v := cpu.GetReg(rs1)
		rs2v := cpu.GetReg(rs2)
		var shouldBranch bool
		switch funct3 {
		case FUNCT_BEQ:
			shouldBranch = rs1v == rs2v
		case FUNCT_BNE:
			shouldBranch = rs1v != rs2v
		case FUNCT_BLT:
			shouldBranch = int32(rs1v) < int32(rs2v)
		case FUNCT_BLTU:
			shouldBranch = rs1v < rs2v
		case FUNCT_BGE:
			shouldBranch = int32(rs1v) >= int32(rs2v)
		case FUNCT_BGEU:
			shouldBranch = rs1v >= rs2v
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}

		if shouldBranch {
			cpu.pc += imm - 4
		}
	case OP_LOAD:
		_, dest, width, base, imm := itype(inst)
		addr := cpu.GetReg(base) + imm
		var res uint32
		switch width {
		case 0: // LB
			res = signExtend(uint32(cpu.LoadByte(addr)), 8)
		case 1: // LH
			res = signExtend(uint32(cpu.LoadHalfWord(addr)), 16)
		case 2: // LW
			res = cpu.LoadWord(addr)
		case 4: // LBU
			res = uint32(cpu.LoadByte(addr))
		case 5: // LHU
			res = uint32(cpu.LoadHalfWord(addr))
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}
		cpu.SetReg(dest, res)
	case OP_STORE:
		_, funct, rs1, rs2, imm := stype(inst)
		addr := cpu.GetReg(rs1) + imm
		rs2v := cpu.GetReg(rs2)
		switch funct {
		case 0: // SB
			cpu.StoreByte(addr, uint8(rs2v))
		case 1: // SH
			cpu.StoreHalfWord(addr, uint16(rs2v))
		case 2: // LW
			cpu.StoreWord(addr, rs2v)
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}
	case OP_SYSTEM:
		_, rd, funct3, rs1, imm := itype(inst)
		switch funct3 {
		case FUNCT_CSRRW, FUNCT_CSRRS, FUNCT_CSRRC:
			csr := imm & 0xfff
			if !cpu.IsValidCsr(csr) {
				trap(ExceptionIllegalInstruction, inst)
				break decode
			}

			// check if we are trying to write to an RO csr
			isReadOnly := csr > 0xc00
			if isReadOnly && rs1 != 0 {
				trap(ExceptionIllegalInstruction, inst)
				break decode
			}

			csrv := cpu.GetCsr(csr)
			rs1v := cpu.GetReg(rs1)
			cpu.SetReg(rd, csrv)
			if rs1 != 0 {
				switch funct3 {
				case FUNCT_CSRRW:
					csrv = rs1v
				case FUNCT_CSRRS:
					csrv = csrv & rs1v
				case FUNCT_CSRRC:
					csrv = csrv & (^rs1v)
				}
				cpu.SetCsr(csr, csrv)
			}
		case FUNCT_PRIV:
			switch imm {
			case PRIV_ECALL:
				trap(ExceptionEcallM, cpu.pc-4)
				break decode
			case PRIV_EBREAK:
				trap(ExceptionBreakpoint, cpu.pc-4)
				break decode
			default:
				trap(ExceptionIllegalInstruction, inst)
				break decode
			}
		default:
			trap(ExceptionIllegalInstruction, inst)
			break decode
		}

	default:
		trap(ExceptionIllegalInstruction, inst)
	}

	cpu.cycles += 1
	cpu.ticks += 1
	cpu.instret += 1
}

func (cpu *Cpu) Step() {
	if cpu.halt {
		return
	}

	inst := cpu.fetch()
	cpu.decode(inst)
}

func bitrange(inst uint32, fromBit, len uint) uint32 {
	return (inst >> fromBit) & ((1 << len) - 1)
}

func signExtend(n uint32, bit uint) uint32 {
	if n&(1<<bit) != 0 {
		n |= ^((1 << bit) - 1)
	}

	return n
}

func btype(inst uint32) (opcode, funct3, rs1, rs2 uint8, imm uint32) {
	imm |= bitrange(inst, 8, 4) << 1
	imm |= bitrange(inst, 25, 6) << 5
	imm |= bitrange(inst, 7, 1) << 11
	imm |= bitrange(inst, 31, 1) << 12
	imm = signExtend(imm, 12)
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 12, 3)),
		uint8(bitrange(inst, 15, 5)),
		uint8(bitrange(inst, 20, 5)),
		imm
}

func itype(inst uint32) (opcode, rd, funct, rs1 uint8, imm uint32) {
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 7, 5)),
		uint8(bitrange(inst, 12, 3)),
		uint8(bitrange(inst, 15, 5)),
		signExtend(bitrange(inst, 20, 12), 11)
}

func rtype(inst uint32) (opcode, rd, funct3, rs1, rs2, funct7 uint8) {
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 7, 5)),
		uint8(bitrange(inst, 12, 3)),
		uint8(bitrange(inst, 15, 5)),
		uint8(bitrange(inst, 20, 5)),
		uint8(bitrange(inst, 25, 7))
}

func utype(inst uint32) (opcode, rd uint8, imm uint32) {
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 7, 5)),
		bitrange(inst, 12, 20)
}

func jtype(inst uint32) (opcode, rd uint8, imm uint32) {
	imm |= bitrange(inst, 21, 10) << 1
	imm |= bitrange(inst, 20, 1) << 11
	imm |= bitrange(inst, 12, 8) << 12
	imm |= bitrange(inst, 31, 1) << 20
	imm = signExtend(imm, 20)
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 7, 5)),
		imm
}

func stype(inst uint32) (opcode, funct3, rs1, rs2 uint8, imm uint32) {
	imm |= bitrange(inst, 7, 5)
	imm |= bitrange(inst, 25, 7) << 5
	imm = signExtend(imm, 11)
	return uint8(bitrange(inst, 0, 7)),
		uint8(bitrange(inst, 12, 3)),
		uint8(bitrange(inst, 15, 5)),
		uint8(bitrange(inst, 20, 5)),
		imm
}

type Board struct {
	cpu *Cpu
}

func (b *Board) Cpu() *Cpu {
	return b.cpu
}

func (b *Board) Execute() {
	b.cpu.Execute()
}

func (b *Board) Step() {
	b.cpu.Step()
}

const BoardInitialAddr = 0x100

func NewBoard(prog []uint8, in io.Reader, out io.Writer) *Board {
	mmu := NewMmu()
	mmu.AddRange(BoardInitialAddr, uint32(len(prog)), NewRamFromBuffer(prog))
	mmu.AddRange(0xfffffffe, 1, &MmioSerial{r: in, w: out})
	cpu := New(mmu, BoardInitialAddr)
	cpu.Reset()
	return &Board{
		cpu: cpu,
	}
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		flag.PrintDefaults()
		os.Exit(1)
	}
	prog, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	board := NewBoard(prog, os.Stdin, os.Stdout)
	board.Execute()
	os.Exit(int(board.Cpu().GetCsr(CsrHalt)))
}
