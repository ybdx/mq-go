package mq

var admin = newWrapper("admin")

type AdminMsg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func Admin() *Queue {
	return admin.Init()
}
