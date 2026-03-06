package domain

import "time"

// Hospital represents the hospital domain entity
type Hospital struct {
	ID           string    `gorm:"column:id;type:varchar(100);primaryKey" json:"id"`
	HosName      string    `gorm:"column:hosName;type:varchar(155);not null" json:"hosName"`
	HosSeq       int       `gorm:"column:hosSeq;uniqueIndex;autoIncrement" json:"hosSeq"`
	HosAddress   *string   `gorm:"column:hosAddress;type:varchar(155)" json:"hosAddress"`
	HosTel       *string   `gorm:"column:hosTel;type:varchar(100)" json:"hosTel"`
	UserContact  *string   `gorm:"column:userContact;type:varchar(155)" json:"userContact"`
	UserTel      *string   `gorm:"column:userTel;type:varchar(100)" json:"userTel"`
	HosLatitude  *string   `gorm:"column:hosLatitude;type:varchar(155)" json:"hosLatitude"`
	HosLongitude *string   `gorm:"column:hosLongitude;type:varchar(155)" json:"hosLongitude"`
	HosPic       *string   `gorm:"column:hosPic;type:varchar(255)" json:"hosPic"`
	HosType      HosType   `gorm:"column:hosType;type:varchar(20);default:'HOSPITAL'" json:"hosType"`
	CreatedAt    time.Time `gorm:"column:createAt;autoCreateTime" json:"createAt"`
	UpdatedAt    time.Time `gorm:"column:updateAt;autoUpdateTime" json:"updateAt"`

	// Relations
	Wards []Ward `gorm:"foreignKey:HosID;references:ID" json:"ward,omitempty"`
}

func (Hospital) TableName() string {
	return "Hospitals"
}
