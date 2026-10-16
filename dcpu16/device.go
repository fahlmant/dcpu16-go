package dcpu16

type Device interface {
	ID() uint32
	Version() uint16
	Manufacturer() uint32
	HandleInterrupt(d *DCPU16)
}
