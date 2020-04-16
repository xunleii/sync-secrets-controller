package controller

type (
	NoAnnotationError struct{ error }
	AnnotationError   struct{ error }
	RegistryError     struct{ error }
	ClientError       struct{ error }
)
