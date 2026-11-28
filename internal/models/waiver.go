package models

import (
	"time"

	"github.com/google/uuid"
)

// WaiverSettings stores the configurable liability waiver displayed on student registration.
type WaiverSettings struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultWaiverSettings returns the baseline Series Taekwondo student liability waiver.
func DefaultWaiverSettings() *WaiverSettings {
	return &WaiverSettings{
		ID:    uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Title: "Series Taekwondo Student Registration & Liability Waiver",
		Content: `1. ACKNOWLEDGMENT OF RISK: I understand that Taekwondo training, sparring, self-defense drills, and physical conditioning involve inherent risks of physical injury. I voluntarily assume all risks associated with participation at Series Taekwondo.

2. MEDICAL CLEARANCE: I certify that the registered student is physically fit to participate in martial arts training and that any relevant medical conditions, allergies, or prior injuries have been disclosed in this registration form.

3. EMERGENCY AUTHORIZATION: In the event of an injury or medical emergency, I authorize Series Taekwondo instructors and staff to administer basic first aid and contact the designated emergency contact or seek emergency medical care if necessary.

4. DOJANG CODE OF CONDUCT: I agree to abide by all Series Taekwondo safety rules, instructor directions, and dojang etiquette at all times.

5. RELEASE OF LIABILITY: By signing below, I (or the parent/legal guardian if the student is a minor) release and hold harmless Series Taekwondo, its owners, coaches, and staff from any liability for injuries or damages arising from ordinary participation in training activities.`,
		UpdatedBy: "System Default",
		UpdatedAt: time.Now(),
	}
}
