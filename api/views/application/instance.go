package application

import "github.com/coroot/coroot-focus/model"

func (i *Instance) addClient(id model.ApplicationId, status model.Status, direction string) {
	for _, a := range i.Clients {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
			}
			return
		}
	}
	for _, a := range i.Dependencies {
		if a.Id == id {
			a.Direction = "both"
			return
		}
	}
	i.Clients = append(i.Clients, &ApplicationLink{Id: id, Status: status, Direction: direction})
}

func (i *Instance) addDependency(id model.ApplicationId, status model.Status, direction string) {
	for _, a := range i.Dependencies {
		if a.Id == id {
			if a.Status < status {
				a.Status = status
			}
			return
		}
	}
	i.Dependencies = append(i.Dependencies, &ApplicationLink{Id: id, Status: status, Direction: direction})
}

func (i *Instance) addInternalLink(id string, status model.Status) {
	for _, l := range i.InternalLinks {
		if l.Id == id {
			return
		}
	}
	i.InternalLinks = append(i.InternalLinks, &InstanceLink{Id: id, Status: status, Direction: "to"})
}
