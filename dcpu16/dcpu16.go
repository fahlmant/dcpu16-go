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
	Registers      [8]uint16
	PC             uint16
	SP             uint16
	EX             uint16
	IA             uint16
	RAM            [0x10000]uint16
	InterruptQueue []uint16
	Queueing       bool
	OnFire         bool
	Devices        []Device
}

func (d *DCPU16) Step() int {

	if d.OnFire {
		return 0
	}

	// Handle interrupts
	d.processInterrupt()
	// Load 16-bit opcode and executes it
	opcode := d.nextWord()
	cycles := d.executeOpcode(opcode)

	return cycles
}

func (d *DCPU16) Interrupt(message uint16) {
	// Enqueue an interrupt with the attached messages
	d.InterruptQueue = append(d.InterruptQueue, message)
	// Absolute limit of the interrupt queue, DCPU catches on fire if breached
	if len(d.InterruptQueue) > 256 {
		d.OnFire = true
	}
}

func (d *DCPU16) processInterrupt() {

	if d.Queueing || len(d.InterruptQueue) == 0 {
		return
	}

	message := d.InterruptQueue[0]
	d.InterruptQueue = d.InterruptQueue[1:]

	if d.IA == 0 {
		return
	}

	// Turn on queueing, save PC then A, jump to IA,
	// put the message in A.
	d.Queueing = true
	d.SP--
	d.RAM[d.SP] = d.PC
	d.SP--
	d.RAM[d.SP] = d.Registers[A]
	d.PC = d.IA
	d.Registers[A] = message

}

func (d *DCPU16) executeOpcode(opcode uint16) int {

	cycles := 0
	// The operation is the bottom 5 bits
	op := opcode & 0x1F

	// if the bottom 5 bits are 0s, this is a special opcode
	if op == 0 {
		cycles += d.executeSpecialOpcode(opcode)
		return cycles
	}

	// Continue with basic opcode handling
	// a is the top 6 bits of the 16-bit word
	a := (opcode >> 10) & 0x3F
	// b is the next 5 bits after a
	b := (opcode >> 5) & 0x1F

	// Resolve the value of a before b's side effects can change it
	aOperand := *d.operand(a, true)
	bOperand := d.operand(b, false)

	switch {
	// SET b, a | sets b to a
	case op == 0x01:
		*bOperand = aOperand
		cycles = 1
	// ADD b, a | sets b to b+a, sets EX to 0x0001 if there's an overflow, 0 otherwise
	case op == 0x02:
		sum := uint32(*bOperand) + uint32(aOperand)
		*bOperand = uint16(sum)
		d.EX = uint16(sum >> 16)
		cycles = 2
	// SUB b, a; sets b to b-a, sets EX to 0xffff if there's an underflow, 0 otherwise
	case op == 0x03:
		diff := uint32(*bOperand) - uint32(aOperand)
		*bOperand = uint16(diff)
		d.EX = uint16(diff >> 16)
		cycles = 2
	// MUL b, a | sets b to b*a, sets EX to ((b*a)>>16)&0xffff
	case op == 0x04:
		product := uint32(*bOperand) * uint32(aOperand)
		*bOperand = uint16(product)
		d.EX = uint16(product>>16) & 0xffff
		cycles = 2
	// MLI b, a | same as MUL, but treat b, a as signed
	case op == 0x05:
		product := int32(int16(*bOperand)) * int32(int16(aOperand))
		*bOperand = uint16(product)
		d.EX = uint16(product>>16) & 0xffff
		cycles = 2
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
		cycles = 3
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
		cycles = 3
	// MOD b, a | sets b to b%a. if a==0, sets b to 0 instead.
	case op == 0x08:
		if aOperand == 0 {
			*bOperand = 0

		} else {
			*bOperand = *bOperand % aOperand
		}
		cycles = 3
	//MDI b, a  | like MOD, but treat b, a as signed.
	case op == 0x09:
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if a == 0 {
			*bOperand = 0
		} else {
			*bOperand = uint16(b % a)
		}
		cycles = 3
	// AND b, a | sets b to b&a
	case op == 0x0a:
		*bOperand = *bOperand & aOperand
		cycles = 1
	// BOR b, a | sets b to b|a
	case op == 0x0b:
		*bOperand = *bOperand | aOperand
		cycles = 1
	// XOR b,a | sets b to b^a
	case op == 0x0c:
		*bOperand = *bOperand ^ aOperand
		cycles = 1
	// SHR b, a | sets b to b>>>a, sets EX to ((b<<16)>>a)&0xffff
	case op == 0x0d:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << 16) >> a) & 0xffff)
		*bOperand = uint16(b >> a)
		cycles = 1
	// ASR b, a | sets b to b>>a, sets EX to ((b<<16)>>>a)&0xffff
	case op == 0x0e:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << 16) >> a) & 0xffff)
		*bOperand = uint16(int16(b) >> a)
		cycles = 1
	// SHL b, a | sets b to b<<a, sets EX to ((b<<a)>>16)&0xffff
	case op == 0x0f:
		a := uint32(aOperand)
		b := uint32(*bOperand)
		d.EX = uint16(((b << a) >> 16) & 0xffff)
		*bOperand = uint16(b << a)
		cycles = 1
	// IFB b, a | performs next instruction only if (b&a)!=0
	case op == 0x10:
		cycles = 2
		if *bOperand&aOperand == 0 {
			cycles += d.skipNextInstruction()
		}
	// IFC b, a | performs next instruction only if (b&a)==0
	case op == 0x11:
		cycles = 2
		if *bOperand&aOperand != 0 {
			cycles += d.skipNextInstruction()
		}
	// IFE b, a | performs next instruction only if b==a
	case op == 0x12:
		cycles = 2
		if *bOperand != aOperand {
			cycles += d.skipNextInstruction()
		}
	// IFN b, a | performs next instruction only if b!=a
	case op == 0x13:
		cycles = 2
		if *bOperand == aOperand {
			cycles += d.skipNextInstruction()
		}
	// IFG b, a | performs next instruction only if b>a
	case op == 0x14:
		cycles = 2
		if *bOperand <= aOperand {
			cycles += d.skipNextInstruction()
		}
	// IFA b, a | performs next instruction only if b>a (signed)
	case op == 0x15:
		cycles = 2
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if b <= a {
			cycles += d.skipNextInstruction()
		}
	// IFL b, a | performs next instruction only if b<a
	case op == 0x16:
		cycles = 2
		if *bOperand >= aOperand {
			cycles += d.skipNextInstruction()
		}
	// IFU b, a | performs next instruction only if b<a (signed)
	case op == 0x17:
		cycles = 2
		a := int32(int16(aOperand))
		b := int32(int16(*bOperand))
		if b >= a {
			cycles += d.skipNextInstruction()
		}
	// ADX b, a | sets b to b+a+EX, sets EX to 0x0001 if there is an over-flow, 0 otherwise
	case op == 0x1A:
		sum := uint32(*bOperand) + uint32(aOperand) + uint32(d.EX)
		*bOperand = uint16(sum)
		d.EX = uint16(sum >> 16)
		cycles = 3
	// SBX b, a | sets b to b-a+EX, sets EX to 0xFFFF if there is an under-flow, 0 otherwise
	case op == 0x1B:
		diff := int32(*bOperand) - int32(aOperand) + int32(int16(d.EX))
		*bOperand = uint16(diff)
		if diff < 0 {
			d.EX = 0xFFFF
		} else {
			d.EX = 0
		}
		cycles = 3
	// STI b, a | sets b to a, then increases I and J by 1
	case op == 0x1E:
		*bOperand = aOperand
		d.Registers[I]++
		d.Registers[J]++
		cycles = 2
	// STD b, a | sets b to a, then decreases I and J by 1
	case op == 0x1F:
		*bOperand = aOperand
		d.Registers[I]--
		d.Registers[J]--
		cycles = 2
	}

	// Operands that read the next word cost one extra cycle each to look up
	if usesNextWord(a) {
		cycles++
	}
	if usesNextWord(b) {
		cycles++
	}

	return cycles
}

func (d *DCPU16) executeSpecialOpcode(opcode uint16) int {

	cycles := 0
	// a is the top 6 bits of the 16-bit word
	a := (opcode >> 10) & 0x3F
	// op is the next 5 bits after a in special opcodes
	op := (opcode >> 5) & 0x1F

	aOperand := d.operand(a, true) // Pointer rather than the value as the special opcodes use the pointer

	switch {
	// JSR a | pushes the address of the next instruction to the stack, then sets PC to a
	case op == 0x1:
		// Resolving the aOperand above already increments PC
		d.SP--
		d.RAM[d.SP] = d.PC
		d.PC = *aOperand
		cycles = 3
	// INT a | triggers a software interrupt with message a
	case op == 0x8:
		d.Interrupt(*aOperand)
		cycles = 4
	// IAG a | sets a to IA
	case op == 0x9:
		*aOperand = d.IA
		cycles = 1
	//  IAS a | sets IA to a
	case op == 0xA:
		d.IA = *aOperand
		cycles = 1
	// RFI a | disables interrupt queueing, pops A from the stack, then pops PC from the stack
	case op == 0xB:
		d.Queueing = false
		d.Registers[A] = d.RAM[d.SP]
		d.SP++
		d.PC = d.RAM[d.SP]
		d.SP++
		cycles = 3
	// IAQ a | if a is nonzero, interrupts will be added to the queue instead of triggered.
	// if a is zero, interrupts will be triggered as normal again
	case op == 0xC:
		d.Queueing = *aOperand != 0
		cycles = 2
	// HWN a | sets a to number of connected hardware devices
	case op == 0x10:
		*aOperand = uint16(len(d.Devices))
		cycles = 2
	// HWQ a | sets A, B, C, X, Y registers to information about hardware a
	// A+(B<<16) is a 32 bit word identifying the hardware id
	// C is the hardware version
	// X+(Y<<16) is a 32 bit word identifying the manufacturer
	case op == 0x11:
		device := d.Devices[*aOperand]
		d.Registers[A] = uint16(device.ID())
		d.Registers[B] = uint16(device.ID() >> 16)
		d.Registers[C] = device.Version()
		d.Registers[X] = uint16(device.Manufacturer())
		d.Registers[Y] = uint16(device.Manufacturer() >> 16)
		cycles = 4
	//  HWI a | sends an interrupt to hardware a
	case op == 0x12:
		device := d.Devices[*aOperand]
		cycles = 4 + device.HandleInterrupt(d)
	}

	if usesNextWord(a) {
		cycles++
	}

	return cycles
}

// Grabs the next 16-bit word from RAM at the PC,
// and increments the PC
func (d *DCPU16) nextWord() uint16 {
	w := d.RAM[d.PC]
	d.PC++
	return w
}

func (d *DCPU16) skipNextInstruction() int {

	skipped := 0
	for {
		skipped++
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
			return skipped
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
