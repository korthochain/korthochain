package typeclass

//Error information is returned when read fails, and [] byte {} and error information are returned when there is an error
type Read interface {
	Read([]byte) ([]byte, error) // Returns an array of unresolved bytes
}
