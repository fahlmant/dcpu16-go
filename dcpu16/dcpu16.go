package dcpu16

import "iter"

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

}

// Grabs the next 16-bit word from RAM at the PC,
// and increments the PC
func (d *DCPU16) nextWord() uint16 {
	w := d.RAM[d.PC]
	d.PC++
	return w
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
