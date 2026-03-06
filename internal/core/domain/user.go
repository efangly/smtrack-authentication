package domain

import "time"

// User represents the user domain entity
type User struct {
	ID        string    `gorm:"column:id;type:varchar(100);primaryKey" json:"id"`
	WardID    string    `gorm:"column:wardId;type:varchar(100);not null" json:"wardId"`
	Username  string    `gorm:"column:username;type:varchar(155);uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password;type:varchar(155)" json:"-"`
	Status    bool      `gorm:"column:status;default:true" json:"status"`
	Role      Role      `gorm:"column:role;type:varchar(20);default:'GUEST'" json:"role"`
	Display   *string   `gorm:"column:display;type:varchar(150)" json:"display"`
	Pic       *string   `gorm:"column:pic;type:varchar(255)" json:"pic"`
	Comment   *string   `gorm:"column:comment;type:varchar(255)" json:"comment"`
	CreateBy  *string   `gorm:"column:createBy;type:varchar(155)" json:"createBy"`
	CreatedAt time.Time `gorm:"column:createAt;autoCreateTime" json:"createAt"`
	UpdatedAt time.Time `gorm:"column:updateAt;autoUpdateTime" json:"updateAt"`

	// Relations
	Ward *Ward `gorm:"foreignKey:WardID;references:ID" json:"ward,omitempty"`
}

func (User) TableName() string {
	return "Users"
}

// UserListItem is a projection for listing users
type UserListItem struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Status   bool    `json:"status"`
	Role     Role    `json:"role"`
	Display  *string `json:"display"`
	Pic      *string `json:"pic"`
	Ward     *struct {
		ID       string `json:"id"`
		WardName string `json:"wardName"`
		HosID    string `json:"hosId"`
	} `json:"ward,omitempty"`
}

// UserDetail is a projection for single user detail
type UserDetail struct {
	ID       string  `json:"id"`
	Username string  `json:"username"`
	Status   bool    `json:"status"`
	Role     Role    `json:"role"`
	Display  *string `json:"display"`
	Pic      *string `json:"pic"`
	WardID   string  `json:"wardId"`
	Ward     *struct {
		WardName string  `json:"wardName"`
		Type     *string `json:"type"`
		HosID    string  `json:"hosId"`
		Hospital *struct {
			HosName string  `json:"hosName"`
			HosPic  *string `json:"hosPic"`
		} `json:"hospital,omitempty"`
	} `json:"ward,omitempty"`
}
