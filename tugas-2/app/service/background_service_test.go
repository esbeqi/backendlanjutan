package service

import "testing"

func TestNotificationTaskIDIsStablePerRecipient(t *testing.T) {
	first := notificationTaskID(42, 7)
	if first != notificationTaskID(42, 7) {
		t.Fatal("task ID untuk penerima yang sama harus stabil")
	}
	if first == notificationTaskID(42, 8) {
		t.Fatal("task ID untuk penerima berbeda tidak boleh sama")
	}
}
