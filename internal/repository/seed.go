package repository

import (
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func (m *MemoryStore) seedData() {
	now := time.Now()

	// 1. Coaches
	c1ID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	c2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	c3ID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	c1Expiry := now.AddDate(1, 0, 0)
	c2Expiry := now.AddDate(0, 0, 10)  // Expires in 10 days (triggers warning!)
	c3Expiry := now.AddDate(0, 0, -15) // Expired 15 days ago!

	c1 := &models.Coach{
		ID:                c1ID,
		FullName:          "Master Dae-Hyun Kim",
		Email:             "master.kim@seriestkd.com",
		Phone:             "+1 (555) 019-2831",
		BeltRank:          "6th Dan Black Belt",
		RatePerSession:    85.00,
		FirstAidCertified: true,
		FirstAidExpiry:    &c1Expiry,
		Specialties:       []string{"Forms/Poomsae", "Sparring/Kyorugi", "Demo Team"},
		IsActive:          true,
		CreatedAt:         now.AddDate(-2, 0, 0),
	}

	c2 := &models.Coach{
		ID:                c2ID,
		FullName:          "Coach Ji-Woo Park",
		Email:             "jiwoo.park@seriestkd.com",
		Phone:             "+1 (555) 014-9920",
		BeltRank:          "4th Dan Black Belt",
		RatePerSession:    65.00,
		FirstAidCertified: true,
		FirstAidExpiry:    &c2Expiry,
		Specialties:       []string{"Sparring/Kyorugi", "Conditioning"},
		IsActive:          true,
		CreatedAt:         now.AddDate(-1, 0, 0),
	}

	c3 := &models.Coach{
		ID:                c3ID,
		FullName:          "Coach Min-Seok Lee",
		Email:             "minseok.lee@seriestkd.com",
		Phone:             "+1 (555) 017-4482",
		BeltRank:          "3rd Dan Black Belt",
		RatePerSession:    55.00,
		FirstAidCertified: false,
		FirstAidExpiry:    &c3Expiry,
		Specialties:       []string{"Cadets/Kids", "Forms/Poomsae"},
		IsActive:          true,
		CreatedAt:         now.AddDate(0, -6, 0),
	}

	m.coaches[c1ID] = c1
	m.coaches[c2ID] = c2
	m.coaches[c3ID] = c3

	// 2. Package Templates
	t1ID := uuid.MustParse("a1111111-1111-1111-1111-111111111111")
	t2ID := uuid.MustParse("a2222222-2222-2222-2222-222222222222")
	t3ID := uuid.MustParse("a3333333-3333-3333-3333-333333333333")

	c12 := 12
	c24 := 24

	t1 := &models.PackageTemplate{
		ID:           t1ID,
		Title:        "12-Session Sparring & Technical Pass",
		SessionCount: &c12,
		ValidityDays: 90,
		Price:        180.00,
		IsActive:     true,
	}

	t2 := &models.PackageTemplate{
		ID:           t2ID,
		Title:        "24-Session Promotion Prep Pass",
		SessionCount: &c24,
		ValidityDays: 180,
		Price:        320.00,
		IsActive:     true,
	}

	t3 := &models.PackageTemplate{
		ID:           t3ID,
		Title:        "Monthly Unlimited Athlete Membership",
		SessionCount: nil, // Unlimited
		ValidityDays: 30,
		Price:        220.00,
		IsActive:     true,
	}

	m.packageTemplates[t1ID] = t1
	m.packageTemplates[t2ID] = t2
	m.packageTemplates[t3ID] = t3

	// 3. Students
	s1ID := uuid.MustParse("b1111111-1111-1111-1111-111111111111") // Ready
	s2ID := uuid.MustParse("b2222222-2222-2222-2222-222222222222") // Pre-test eligible
	s3ID := uuid.MustParse("b3333333-3333-3333-3333-333333333333") // Developing
	s4ID := uuid.MustParse("b4444444-4444-4444-4444-444444444444") // High Dan candidate

	s1 := &models.Student{
		ID:                s1ID,
		FullName:          "Alex Vance",
		DOB:               now.AddDate(-14, 0, 0),
		Gender:            "Male",
		Phone:             "+1 (555) 234-5678",
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: now.AddDate(0, 0, -65), // 65 days in rank (req: 45)
		EmergencyName:     "Sarah Vance",
		EmergencyPhone:    "+1 (555) 234-5679",
		EmergencyRelation: "Mother",
		MedicalNotes:      "Mild asthma, uses inhaler before intense cardio",
		IsActive:          true,
		CreatedAt:         now.AddDate(0, -3, 0),
	}

	s2 := &models.Student{
		ID:                s2ID,
		FullName:          "Chloe Ramirez",
		DOB:               now.AddDate(-16, 0, 0),
		Gender:            "Female",
		Phone:             "+1 (555) 345-6789",
		CurrentBelt:       models.BeltYellow,
		LastPromotionDate: now.AddDate(0, 0, -75), // 75 days in rank (req: 60)
		EmergencyName:     "Carlos Ramirez",
		EmergencyPhone:    "+1 (555) 345-6780",
		EmergencyRelation: "Father",
		MedicalNotes:      "No known allergies or medical restrictions",
		IsActive:          true,
		CreatedAt:         now.AddDate(0, -5, 0),
	}

	s3 := &models.Student{
		ID:                s3ID,
		FullName:          "Marcus Brody",
		DOB:               now.AddDate(-12, 0, 0),
		Gender:            "Male",
		Phone:             "+1 (555) 456-7890",
		CurrentBelt:       models.BeltGreenTag,
		LastPromotionDate: now.AddDate(0, 0, -15), // Only 15 days in rank!
		EmergencyName:     "Elena Brody",
		EmergencyPhone:    "+1 (555) 456-7891",
		EmergencyRelation: "Mother",
		MedicalNotes:      "Previous wrist sprain, clear for non-contact forms",
		IsActive:          true,
		CreatedAt:         now.AddDate(0, -2, 0),
	}

	s4 := &models.Student{
		ID:                s4ID,
		FullName:          "Sophia Chen",
		DOB:               now.AddDate(-18, 0, 0),
		Gender:            "Female",
		Phone:             "+1 (555) 567-8901",
		CurrentBelt:       models.BeltRed,
		LastPromotionDate: now.AddDate(0, 0, -160),
		EmergencyName:     "David Chen",
		EmergencyPhone:    "+1 (555) 567-8902",
		EmergencyRelation: "Father",
		MedicalNotes:      "Full medical clearance",
		IsActive:          true,
		CreatedAt:         now.AddDate(-2, 0, 0),
	}

	m.students[s1ID] = s1
	m.students[s2ID] = s2
	m.students[s3ID] = s3
	m.students[s4ID] = s4

	// 4. Student Packages
	sp1Rem := 8
	sp1 := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         s1ID,
		TemplateID:        t1ID,
		TemplateTitle:     t1.Title,
		TotalSessions:     t1.SessionCount,
		RemainingSessions: &sp1Rem,
		PurchaseDate:      now.AddDate(0, 0, -30),
		ExpiryDate:        now.AddDate(0, 0, 60),
		PaymentStatus:     "paid",
		CreatedAt:         now.AddDate(0, 0, -30),
	}

	sp2Rem := 15
	sp2 := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         s2ID,
		TemplateID:        t2ID,
		TemplateTitle:     t2.Title,
		TotalSessions:     t2.SessionCount,
		RemainingSessions: &sp2Rem,
		PurchaseDate:      now.AddDate(0, 0, -60),
		ExpiryDate:        now.AddDate(0, 0, 120),
		PaymentStatus:     "paid",
		CreatedAt:         now.AddDate(0, 0, -60),
	}

	sp3Rem := 2
	sp3 := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         s3ID,
		TemplateID:        t1ID,
		TemplateTitle:     t1.Title,
		TotalSessions:     t1.SessionCount,
		RemainingSessions: &sp3Rem,
		PurchaseDate:      now.AddDate(0, 0, -10),
		ExpiryDate:        now.AddDate(0, 0, 80),
		PaymentStatus:     "paid",
		CreatedAt:         now.AddDate(0, 0, -10),
	}

	sp4 := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         s4ID,
		TemplateID:        t3ID,
		TemplateTitle:     t3.Title,
		TotalSessions:     nil,
		RemainingSessions: nil,
		PurchaseDate:      now.AddDate(0, 0, -15),
		ExpiryDate:        now.AddDate(0, 0, 15),
		PaymentStatus:     "paid",
		CreatedAt:         now.AddDate(0, 0, -15),
	}

	m.studentPackages[sp1.ID] = sp1
	m.studentPackages[sp2.ID] = sp2
	m.studentPackages[sp3.ID] = sp3
	m.studentPackages[sp4.ID] = sp4

	// 5. Training Sessions
	sess1ID := uuid.MustParse("c1111111-1111-1111-1111-111111111111")
	sess2ID := uuid.MustParse("c2222222-2222-2222-2222-222222222222")

	sess1 := &models.TrainingSession{
		ID:           sess1ID,
		SessionDate:  now,
		StartTime:    "17:00",
		EndTime:      "18:30",
		CoachID:      c1ID,
		CoachName:    c1.FullName,
		AdminID:      &c1ID,
		AdminName:    c1.FullName,
		TrainingType: models.TrainingSparring,
		Notes:        "High intensity floor drills, electronic scoring pad practice",
		CreatedAt:    now.Add(-2 * time.Hour),
	}

	sess2 := &models.TrainingSession{
		ID:           sess2ID,
		SessionDate:  now.AddDate(0, 0, -1),
		StartTime:    "18:30",
		EndTime:      "20:00",
		CoachID:      c2ID,
		CoachName:    c2.FullName,
		AdminID:      &c1ID,
		AdminName:    c1.FullName,
		TrainingType: models.TrainingPoomsae,
		Notes:        "Taegeuk 1 through 8 refinement & stance balance auditing",
		CreatedAt:    now.AddDate(0, 0, -1),
	}

	m.sessions[sess1ID] = sess1
	m.sessions[sess2ID] = sess2

	// 6. Seed Attendances for Student 1 (Alex) so he hits 18 sessions
	for i := 0; i < 18; i++ {
		sID := uuid.New()
		tType := models.TrainingPoomsae
		if i%2 == 0 {
			tType = models.TrainingSparring
		}
		dummySess := &models.TrainingSession{
			ID:           sID,
			SessionDate:  now.AddDate(0, 0, -i*2),
			StartTime:    "17:00",
			EndTime:      "18:15",
			CoachID:      c1ID,
			TrainingType: tType,
			Notes:        "Regular class attendance log",
		}
		m.sessions[sID] = dummySess

		att := &models.Attendance{
			ID:               uuid.New(),
			SessionID:        sID,
			StudentID:        s1ID,
			StudentName:      s1.FullName,
			StudentBelt:      s1.CurrentBelt,
			StudentPackageID: &sp1.ID,
			PackageTitle:     t1.Title,
			CheckedInAt:      dummySess.SessionDate,
		}
		m.attendances[att.ID] = att
	}

	// Also check Alex in to current live sess1
	attLive1 := &models.Attendance{
		ID:               uuid.New(),
		SessionID:        sess1ID,
		StudentID:        s1ID,
		StudentName:      s1.FullName,
		StudentBelt:      s1.CurrentBelt,
		StudentPackageID: &sp1.ID,
		PackageTitle:     t1.Title,
		CheckedInAt:      now.Add(-30 * time.Minute),
	}
	m.attendances[attLive1.ID] = attLive1

	// Seed 25 attendances for Chloe (s2)
	for i := 0; i < 25; i++ {
		sID := uuid.New()
		dummySess := &models.TrainingSession{
			ID:           sID,
			SessionDate:  now.AddDate(0, 0, -i*2),
			StartTime:    "18:30",
			EndTime:      "19:45",
			CoachID:      c2ID,
			TrainingType: models.TrainingPoomsae,
			Notes:        "Regular class",
		}
		m.sessions[sID] = dummySess

		att := &models.Attendance{
			ID:               uuid.New(),
			SessionID:        sID,
			StudentID:        s2ID,
			StudentName:      s2.FullName,
			StudentBelt:      s2.CurrentBelt,
			StudentPackageID: &sp2.ID,
			PackageTitle:     t2.Title,
			CheckedInAt:      dummySess.SessionDate,
		}
		m.attendances[att.ID] = att
	}

	// 7. Student Evaluation for Alex (s1)
	eval1 := &models.StudentEvaluation{
		ID:             uuid.New(),
		StudentID:      s1ID,
		CoachID:        c1ID,
		CoachName:      c1.FullName,
		EvaluationDate: now.AddDate(0, 0, -5),
		Flexibility:    8,
		Stamina:        9,
		Power:          7,
		Technique:      8,
		SparringIQ:     8,
		Discipline:     9,
		CoachRemarks:   "Exceptional discipline and kick height. Clear candidate for Yellow Tag promotion testing.",
		CreatedAt:      now.AddDate(0, 0, -5),
	}
	m.evaluations[eval1.ID] = eval1
}
