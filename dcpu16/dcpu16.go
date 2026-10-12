package dcpu16

const (
	A = iota
	B
	C
	X
	Y
	Z
	I
	J
)

// Implements a DCPU-16 according to
// https://github.com/fahlmant/dcpu-specifications/blob/e154827c56664ad4a92ac2d404c6c21583ffee04/dcpu16.txt

type DCPU16 struct {
	Registers [8]uint16
	PC        uint16
	SP        uint16
	EX        uint16
	IA        uint16
	RAM       [0x10000]uint16
}

func (d *DCPU16) Step() {
	// Load 16-bit opcode and executes it
	opcode := d.nextWord()
	d.executeOpcode(opcode)
}

func (d *DCPU16) executeOpcode(opcode uint16) {

	// a is the top 6 bits of the 16-bit word
	a := (opcode >> 10) & 0x3F
	// b is the next 5 bits after a
	b := (opcode >> 5) & 0x1F
	// The operation is the bottom 5 bits
	op := opcode & 0x1F

	// Resolve the value of a before b's side effects can change it
	aOperand := *d.operand(a, true)
	bOperand := d.operand(b, false)

	switch {
	// SET b, a | sets b to a
	case op == 0x01:
		*bOperand = aOperand
	// ADD b, a | sets b to b+a, sets EX to 0x0001 if there's an overflow, 0 otherwise
	case op == 0x02:
		sum := uint32(*bOperand) + uint32(aOperand)
		*bOperand = uint16(sum)
		d.EX = uint16(sum >> 16)
	// SUB b, a; sets b to b-a, sets EX to 0xffff if there's an underflow, 0 otherwise
	case op == 0x03:
		diff := uint32(*bOperand) - uint32(aOperand)
		*bOperand = uint16(diff)
		d.EX = uint16(diff >> 16)
	// MUL b, a | sets b to b*a, sets EX to ((b*a)>>16)&0xffff
	case op == 0x04:
		product := uint32(*bOperand) * uint32(aOperand)
		*bOperand = uint16(product)
		d.EX = uint16(product>>16) & 0xffff
	// MLI b, a | same as MUL, but treat b, a as signed
	case op == 0x05:
		product := int32(int16(*bOperand)) * int32(int16(aOperand))
		*bOperand = uint16(product)
		d.EX = uint16(product>>16) & 0xffff
	// DIV b,a | sets b to b/a, sets EX to ((b<<16)/a)&0xffff. if a==0, sets b and EX to 0
	case op == 0x06:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		if a == 0 {
			*bOperand = 0
			d.EX = 0
		} else {
			diff := b / a
			*bOperand = uint16(diff)
			d.EX = uint16(((b << 16) / a) & 0xffff)
		}
	// DVI b, a | like DIV, but treat b, a as signed. Rounds towards 0
	case op == 0x07:
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if a == 0 {
			*bOperand = 0
			d.EX = 0
		} else {
			diff := b / a
			*bOperand = uint16(diff)
			d.EX = uint16(((b << 16) / a) & 0xffff)
		}
	// MOD b, a | sets b to b%a. if a==0, sets b to 0 instead.
	case op == 0x08:
		if aOperand == 0 {
			*bOperand = 0

		} else {
			*bOperand = *bOperand % aOperand
		}
	//MDI b, a  | like MOD, but treat b, a as signed.
	case op == 0x09:
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if a == 0 {
			*bOperand = 0
		} else {
			*bOperand = uint16(b % a)
		}
	// AND b, a | sets b to b&a
	case op == 0x0a:
		*bOperand = *bOperand & aOperand
	// BOR b, a | sets b to b|a
	case op == 0x0b:
		*bOperand = *bOperand | aOperand
	// XOR b,a | sets b to b^a
	case op == 0x0c:
		*bOperand = *bOperand ^ aOperand
	// SHR b, a | sets b to b>>>a, sets EX to ((b<<16)>>a)&0xffff
	case op == 0x0d:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << 16) >> a) & 0xffff)
		*bOperand = uint16(b >> a)
	// ASR b, a | sets b to b>>a, sets EX to ((b<<16)>>>a)&0xffff
	case op == 0x0e:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << 16) >> a) & 0xffff)
		*bOperand = uint16(int16(b) >> a)
	// SHL b, a | sets b to b<<a, sets EX to ((b<<a)>>16)&0xffff
	case op == 0x0f:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << a) >> 16) & 0xffff)
		*bOperand = uint16(b << a)
	// IFB b, a | performs next instruction only if (b&a)!=0
	case op == 0x10:
		if *bOperand&aOperand == 0 {
			d.skipNextInstruction()
		}
	// IFC b, a | performs next instruction only if (b&a)==0
	case op == 0x11:
		if *bOperand&aOperand != 0 {
			d.skipNextInstruction()
		}
	// IFE b, a | performs next instruction only if b==a
	case op == 0x12:
		if *bOperand != aOperand {
			d.skipNextInstruction()
		}
	// IFN b, a | performs next instruction only if b!=a
	case op == 0x13:
		if *bOperand == aOperand {
			d.skipNextInstruction()
		}
	// IFG b, a | performs next instruction only if b>a
	case op == 0x14:
		if *bOperand <= aOperand {
			d.skipNextInstruction()
		}
	// IFA b, a | performs next instruction only if b>a (signed)
	case op == 0x15:
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if b <= a {
			d.skipNextInstruction()
		}
	// IFL b, a | performs next instruction only if b<a
	case op == 0x16:
		if *bOperand >= aOperand {
			d.skipNextInstruction()
		}
	// IFU b, a | performs next instruction only if b<a (signed)
	case op == 0x17:
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if b >= a {
			d.skipNextInstruction()
		}
	// ADX b, a | sets b to b+a+EX, sets EX to 0x0001 if there is an over-flow, 0 otherwise
	case op == 0x1A:
		sum := uint32(*bOperand) + uint32(aOperand) + uint32(d.EX)
		*bOperand = uint16(sum)
		d.EX = uint16(sum >> 16)
	// SBX b, a | sets b to b-a+EX, sets EX to 0xFFFF if there is an under-flow, 0 otherwise
	case op == 0x1B:
		diff := int32(*bOperand) - int32(aOperand) + int32(int16(d.EX))
		*bOperand = uint16(diff)
		if diff < 0 {
			d.EX = 0xFFFF
		} else {
			d.EX = 0
		}
	// STI b, a | sets b to a, then increases I and J by 1
	case op == 0x1E:
		*bOperand = aOperand
		d.Registers[I]++
		d.Registers[J]++
	// STD b, a | sets b to a, then decreases I and J by 1
	case op == 0x1F:
		*bOperand = aOperand
		d.Registers[I]--
		d.Registers[J]--
	}
}

// Grabs the next 16-bit word from RAM at the PC,
// and increments the PC
func (d *DCPU16) nextWord() uint16 {
	w := d.RAM[d.PC]
	d.PC++
	return w
}

func (d *DCPU16) skipNextInstruction() {

	for {
		// Load the next work and move the PC past it
		word := d.nextWord()

		// Extract the parts of the opcode
		op := word & 0x1F
		a := (word >> 10) & 0x3F
		b := (word >> 5) & 0x1F

		// a is always an operand
		if usesNextWord(a) {
			d.PC++
		}

		// For special instructions (op == 0), the b field is an opcode, not an operand
		if op != 0 && usesNextWord(b) {
			d.PC++
		}
		// A non-conditional ends the chain
		if op < 0x10 || op > 0x17 {
			return
		}
	}
}

// Reports whether an operand code is followed by an extra word
func usesNextWord(op uint16) bool {
	return (op >= 0x10 && op <= 0x17) || // [register + next word]
		op == 0x1A || // PICK n
		op == 0x1E || // [next word]
		op == 0x1F // next word (literal)
}

func (d *DCPU16) operand(input uint16, isA bool) *uint16 {

	switch {
	// Gets value in the registers
	case input <= 0x07:
		return &d.Registers[input]
	//  Gets value in RAM at the address stored in a register
	case input <= 0x0F:
		return &d.RAM[d.Registers[input-0x08]]
	// Gets value in RAM at the address that is the sum
	// of the value stored in a register added with the
	// next word in the program
	case input <= 0x17:
		return &d.RAM[d.Registers[input-0x10]+d.nextWord()]
	// POP and PUSH
	case input == 0x18:
		// If is in a, performs a POP, i.e.[SP++]
		if isA {
			v := &d.RAM[d.SP]
			d.SP++
			return v
		}
		// If in b, performs a PUSH, i.e. [--SP]
		d.SP--
		v := &d.RAM[d.SP]
		return v
	// PEEK at the current value at the stack pointer
	case input == 0x19:
		return &d.RAM[d.SP]
	// PEEK at the nth value on the stack
	case input == 0x1A:
		return &d.RAM[d.SP+d.nextWord()]
	// The Stack Pointer
	case input == 0x1B:
		return &d.SP
	// The Program Counter
	case input == 0x1C:
		return &d.PC
	// The Extra register
	case input == 0x1D:
		return &d.EX
	// Gets the value in RAM at the address of the next word in the program
	case input == 0x1E:
		return &d.RAM[d.nextWord()]
	case input == 0x1F:
		v := d.nextWord()
		return &v
	default:
		v := input - 0x21
		return &v
	}
}
