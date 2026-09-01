package domain

import (
	"reflect"
	"testing"
)

func TestPersistedTypesDeclareFirestoreTags(t *testing.T) {
	for _, value := range []any{MeetingType{}, WeekdayHours{}, CalendarConnection{}, CalendarInvitation{}, Booking{}} {
		typeOf := reflect.TypeOf(value)
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			if field.PkgPath == "" && field.Tag.Get("firestore") == "" {
				t.Errorf("%s.%s has no firestore tag", typeOf.Name(), field.Name)
			}
		}
	}
}
