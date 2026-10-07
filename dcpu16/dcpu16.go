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

type DCPU16 struct {
	Registers [8]uint16
	PC        uint16
	SP        uint16
	EX        uint16
	IA        uint16
	RAM       [0x10000]uint16
}

func (d *DCPU16) Step() {
	// Load 16-bit opcode
	opcode := d.RAM[d.PC]
	// Increment the PC now that the opcode has been read
	d.PC += 1
	d.executeOpcode(opcode)
}

func (d *DCPU16) executeOpcode(opcode uint16) {

	// a is the first 6 bits of the 16-bit word
	a := (opcode >> 10) & 0x3F
	// b is the next 5 bits after a
	b := (opcode >> 5) & 0x1F
	// The operation is the last 5 bits
	op := opcode & 0x1F

}
