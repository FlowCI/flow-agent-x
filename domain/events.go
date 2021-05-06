package domain

type AppEvent string

const EventOnIdle = AppEvent("EventOnIdle")
const EventOnBusy = AppEvent("EventOnBusy")
