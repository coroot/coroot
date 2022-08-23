package model

type ApplicationType string

type Application struct {
	Id ApplicationId

	Instances []*Instance
}

func NewApplication(id ApplicationId) *Application {
	app := &Application{Id: id}
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
		instance = NewInstance(name, app.Id)
		app.Instances = append(app.Instances, instance)
	}
	return instance
}

func (app *Application) Labels() Labels {
	return nil
}
