package base

type UnionBase interface {
	DestroyRoom(roomId string)
	GetOwnerUid() string
	IsOpening() bool
}
