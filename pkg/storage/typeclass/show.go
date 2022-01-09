package typeclass

//When show fails, [] byte {}
type Show interface {
	Show() []byte
}
