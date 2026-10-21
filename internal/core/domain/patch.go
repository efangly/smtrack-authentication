package domain

// The *Patch types are partial updates: a nil field is left unchanged, so an
// update can never zero a field the client did not send.

// UserPatch is a partial update of a user.
type UserPatch struct {
	WardID   *string
	Username *string
	Status   *bool
	Role     *Role
	Display  *string
	Pic      *string
	Comment  *string
	// Password holds the plain value; userService.Update hashes it before it
	// reaches the repository.
	Password *string
}

// WardPatch is a partial update of a ward.
type WardPatch struct {
	WardName *string
	WardSeq  *int
	Type     *WardType
	HosID    *string
}

// HospitalPatch is a partial update of a hospital.
type HospitalPatch struct {
	HosName      *string
	HosAddress   *string
	HosTel       *string
	UserContact  *string
	UserTel      *string
	HosLatitude  *string
	HosLongitude *string
	HosPic       *string
	HosType      *HosType
}
