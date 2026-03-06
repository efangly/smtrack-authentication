package domain

import "time"

// Ward represents the ward domain entity
type Ward struct {
	ID        string    `gorm:"column:id;type:varchar(100);primaryKey" json:"id"`
	WardName  string    `gorm:"column:wardName;type:varchar(250);not null" json:"wardName"`
	WardSeq   int       `gorm:"column:wardSeq;uniqueIndex;autoIncrement" json:"wardSeq"`
	Type      *WardType `gorm:"column:type;type:varchar(20);default:'NEW'" json:"type"`
	HosID     string    `gorm:"column:hosId;type:varchar(100);not null" json:"hosId"`
	CreatedAt time.Time `gorm:"column:createAt;autoCreateTime" json:"createAt"`
	UpdatedAt time.Time `gorm:"column:updateAt;autoUpdateTime" json:"updateAt"`

	// Relations
	Hospital *Hospital `gorm:"foreignKey:HosID;references:ID" json:"hospital,omitempty"`
	Users    []User    `gorm:"foreignKey:WardID;references:ID" json:"user,omitempty"`
}

func (Ward) TableName() string {
	return "Wards"
}
