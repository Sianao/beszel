package migrations

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	names := []string{"UPSCharge", "UPSRuntime", "UPSLoad", "UPSOnBattery", "UPSDisconnected", "UPSFault"}
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("alerts")
		if err != nil {
			return err
		}
		field := collection.Fields.GetByName("name").(*core.SelectField)
		for _, name := range names {
			if !slices.Contains(field.Values, name) {
				field.Values = append(field.Values, name)
			}
		}
		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("alerts")
		if err != nil {
			return err
		}
		field := collection.Fields.GetByName("name").(*core.SelectField)
		field.Values = slices.DeleteFunc(field.Values, func(name string) bool { return slices.Contains(names, name) })
		return app.Save(collection)
	})
}
