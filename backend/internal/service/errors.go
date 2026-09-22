package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrForbidden         = errors.New("role is not permitted for this operation")
	ErrReviewRequired    = errors.New("reviewer role is required for the final signoff decision")
	ErrLocked            = errors.New("record is locked after peer review begins")
	ErrPreparationOwner  = errors.New("only the original preparer may edit or submit this draft")
	ErrSeparationOfDuty  = errors.New("preparer and reviewer must be different users")
)
