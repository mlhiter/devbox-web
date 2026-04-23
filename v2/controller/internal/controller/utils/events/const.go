package events

const (
	ReasonStorageCleanupRequested = "storage-cleanup-requested"
	ReasonDevboxStateChanged      = "devbox-state-changed"

	KeyAnnotationReason         = "reason"
	KeyAnnotationDevboxName     = "devbox-name"
	KeyAnnotationContentID      = "content-id"
	KeyAnnotationBaseImage      = "base-image"
	KeyAnnotationStorageLimit   = "storage-limit"
	KeyAnnotationSnapshotter    = "snapshotter"
	KeyAnnotationRuntimeClass   = "runtime-class"
	KeyAnnotationRuntimeHandler = "runtime-handler"
)

type Annotations map[string]string

func BuildStorageCleanupAnnotations(
	devboxName, contentID, baseImage, storageLimit, snapshotter, runtimeClass, runtimeHandler string,
) Annotations {
	return Annotations{
		KeyAnnotationReason:         ReasonStorageCleanupRequested,
		KeyAnnotationDevboxName:     devboxName,
		KeyAnnotationContentID:      contentID,
		KeyAnnotationBaseImage:      baseImage,
		KeyAnnotationStorageLimit:   storageLimit,
		KeyAnnotationSnapshotter:    snapshotter,
		KeyAnnotationRuntimeClass:   runtimeClass,
		KeyAnnotationRuntimeHandler: runtimeHandler,
	}
}
