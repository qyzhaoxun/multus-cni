package backend

type CNIStore interface {
	Save(id string, data []byte) error
	Load(id string) ([]byte, error)
	Remove(id string) error
}
