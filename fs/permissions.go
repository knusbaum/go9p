package fs

import (
	"github.com/knusbaum/go9p2/proto"
)

const (
	UGO_user  = iota
	UGO_group = iota
	UGO_other = iota
)

func UserInGroup(user string, group string) bool {
	// For now groups and users are equivalent.
	return user == group
}

func UserRelation(user string, f FSNode) uint8 {
	st := f.Stat()
	if user == st.Uid {
		return UGO_user
	}
	if UserInGroup(user, st.Gid) {
		return UGO_group
	}
	return UGO_other
}

func omodePermits(perm uint8, omode uint8) bool {
	switch omode {
	case proto.Oread:
		return perm&0x4 != 0
		break
	case proto.Owrite:
		return perm&0x2 != 0
		break
	case proto.Ordwr:
		return (perm&0x2 != 0) && (perm&0x4 != 0)
		break
	case proto.Oexec:
		return perm&0x01 != 0
		break
	case proto.None:
		return false
		break
	default:
		return false
		break
	}
	return false
}

func OpenPermission(f FSNode, user string, omode uint8) bool {
	switch UserRelation(user, f) {
	case UGO_user:
		return omodePermits(uint8(f.Stat().Mode>>6)&0x07, omode)
		break
	case UGO_group:
		return omodePermits(uint8(f.Stat().Mode>>3)&0x07, omode)
		break
	case UGO_other:
		return omodePermits(uint8(f.Stat().Mode)&0x07, omode)
		break
	default:
		return false
	}
	return false
}