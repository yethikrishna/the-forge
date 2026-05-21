package crossdevice

import (
	"testing"
)

func TestRegisterDevice(t *testing.T) {
	cd := New()
	err := cd.RegisterDevice(DeviceProfile{ID: "laptop-1", Type: DeviceLaptop})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncNewBlobs(t *testing.T) {
	cd := New()
	cd.RegisterDevice(DeviceProfile{ID: "laptop-1", Type: DeviceLaptop})

	blobs := []ContextBlob{
		{ID: "conv-1", Type: BlobConversation, Data: []byte("hello")},
		{ID: "mem-1", Type: BlobMemory, Data: []byte("remember this")},
	}

	result, err := cd.Sync("laptop-1", blobs)
	if err != nil {
		t.Fatal(err)
	}
	if result.Uploaded != 2 {
		t.Errorf("expected 2 uploads, got %d", result.Uploaded)
	}
}

func TestSyncNoChanges(t *testing.T) {
	cd := New()
	cd.RegisterDevice(DeviceProfile{ID: "phone-1", Type: DevicePhone})

	blob := ContextBlob{ID: "conv-1", Version: 1, Data: []byte("hello")}
	cd.Sync("phone-1", []ContextBlob{blob})

	// Same device syncs same blob — no upload
	result, _ := cd.Sync("phone-1", []ContextBlob{blob})
	if result.Uploaded != 0 {
		t.Error("same blob should not be re-uploaded")
	}
}

func TestSetActive(t *testing.T) {
	cd := New()
	cd.RegisterDevice(DeviceProfile{ID: "laptop-1", Type: DeviceLaptop})
	cd.RegisterDevice(DeviceProfile{ID: "phone-1", Type: DevicePhone})

	cd.SetActive("laptop-1")
	if cd.PrimaryDevice().ID != "laptop-1" {
		t.Error("laptop should be primary")
	}

	cd.SetActive("phone-1")
	if cd.PrimaryDevice().ID != "phone-1" {
		t.Error("phone should now be primary")
	}
	if cd.devices["laptop-1"].IsActive {
		t.Error("laptop should be deactivated")
	}
}

func TestGetContext(t *testing.T) {
	cd := New()
	cd.RegisterDevice(DeviceProfile{ID: "laptop-1", Type: DeviceLaptop})
	cd.RegisterDevice(DeviceProfile{ID: "phone-1", Type: DevicePhone})

	// Laptop syncs a blob
	cd.Sync("laptop-1", []ContextBlob{
		{ID: "conv-1", Type: BlobConversation, Data: []byte("important context")},
	})

	// Phone should get the blob
	blobs, err := cd.GetContext("phone-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(blobs) != 1 {
		t.Errorf("expected 1 blob for phone, got %d", len(blobs))
	}
}
