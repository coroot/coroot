package model

type ApplicationType string

type Application struct {
	ApplicationId ApplicationId

	Instances []*Instance
}

func NewApplication(id ApplicationId) *Application {
	app := &Application{ApplicationId: id}
	return app
}

func (app *Application) GetInstance(name string) *Instance {
	for _, i := range app.Instances {
		if i.Name == name {
			return i
		}
	}
	return nil
}

func (app *Application) GetOrCreateInstance(name string) *Instance {
	instance := app.GetInstance(name)
	if instance == nil {
		instance = NewInstance(name, app.ApplicationId)
		app.Instances = append(app.Instances, instance)
	}
	return instance
}

func (app *Application) Labels() Labels {
	return nil
}
