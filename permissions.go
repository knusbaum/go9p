package go9p

const (
	ugo_user = iota
	ugo_group = iota
	ugo_other = iota
)

func UserInGroup(user string, group string) bool {
	// For now groups and users are equivalent.
	return user == group
}

func UserRelation(user string, file *File) uint8 {
	if user == file.Stat.Uid {
		return ugo_user
	}
	if UserInGroup(user, file.Stat.Gid) {
		return ugo_group
	}
	return ugo_other
}

func OmodePermits(perm uint8, omode uint8) bool {
	switch omode {
	case Oread:
		return perm & 0x4 != 0
		break
	case Owrite:
		return perm & 0x2 != 0
		break
	case Ordwr:
		return perm & 0x06 != 0
		break
	case Oexec:
		return perm & 0x01 != 0
		break
	case None:
		return false
		break
	default:
		return false
		break
	}
	return false
}

func OpenPermission(user string, file *File, omode uint8) bool {
	switch(UserRelation(user, file)) {
	case ugo_user:
		return OmodePermits(uint8(file.Stat.Mode >> 6) & 0x07, omode)
		break
	case ugo_group:
		return OmodePermits(uint8(file.Stat.Mode >> 3) & 0x07, omode)
		break
	case ugo_other:
		return OmodePermits(uint8(file.Stat.Mode) & 0x07, omode)
		break
	default:
		return false
	}
	return false
}
