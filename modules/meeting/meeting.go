package meeting

import (
	"fmt"
	iface "github.com/opesun/nocrud/frame/interfaces"
	"github.com/opesun/nocrud/modules/meeting/evenday"
	"time"
)

func isProfessional(u iface.User) bool {
	_, ok := u.Data()["professional"]
	return ok
}

type shared struct {
	db						iface.Db
	userId 					iface.Id
	userIsProfessional		bool
	optDoc					iface.NestedData
	timeTableColl			string
	intervalColl			string
	gotOptions				bool
}

type Entries struct {
	shared
}

func (e *Entries) Init(ctx iface.Context) {
	e.db = ctx.Db()
	e.userId = ctx.User().Id()
	e.userIsProfessional = isProfessional(ctx.User())
	e.optDoc = ctx.Options().Document()
	e.timeTableColl = "timeTables"
	e.intervalColl = "intervals"
}

// This way professionals will only see entries posted for them,
// and users will only see entries posted by them.
// However, a professional will not be able to act as a user: he wont see entries posted by him, posted for other professionals.
func (e *Entries) TopModFilter(a iface.Filter) {
	if e.userIsProfessional {
		a.AddQuery(map[string]interface{}{
			"forProfessional": e.userId,
		})
	} else {
		a.AddQuery(map[string]interface{}{
			"createdBy": e.userId,
		})
	}
}

func (e *Entries) getOptions(resource string) {
	if e.gotOptions {
		return
	}
	e.gotOptions = true
	ttColl, ok := e.optDoc.GetStr("nouns." + resource + ".options.timeTableColl")
	if ok {
		e.timeTableColl = ttColl
	}
	iColl, ok := e.optDoc.GetStr("nouns." + resource + ".options.intervalColl")
	if ok {
		e.intervalColl = iColl
	}
}

func dateToString(u int64) string {
	return time.Unix(u, 0).Format("2006.01.02")
}

func (e *Entries) getTaken(a iface.Filter, from int64) (evenday.DaySchedule, error) {
	b := a.Clone()
	q := map[string]interface{}{
		"day": dateToString(from),
	}
	b.AddQuery(q)
	res, err := b.Find()
	if err != nil {
		return evenday.DaySchedule{}, err
	}
	gen := []interface{}{}
	for _, v := range res {
		gen = append(gen, v)
	}
	return evenday.GenericToDaySchedule(gen)
}

// Returns the closest interval on the same day to the given interval.
func (e *Entries) GetClosest(a iface.Filter, data map[string]interface{}) (*evenday.Interval, error) {
	e.getOptions(a.Subject())
	prof, err := e.db.ToId(data["professional"].(string))
	if err != nil {
		return nil, err
	}
	from := data["from"].(int64)
	length := data["length"].(int64)
	err = e.intervalIsValid(data, prof, length)
	if err != nil {
		return nil, err
	}
	tt, err := e.getTimeTable(prof)
	if err != nil {
		return nil, err
	}
	day := evenday.DateToDayName(from)
	open := tt[day]
	taken, err := e.getTaken(a, from)
	if err != nil {
		return nil, err
	}
	adv := evenday.NewAdvisor(open, taken)
	to := from + length * 60
	adv.Amount(1)
	interv, err := evenday.NewInterval(evenday.DateToMinute(from), evenday.DateToMinute(to))
	if err != nil {
		return nil, err
	}
	ret := adv.Advise(interv)
	if len(ret) == 0 {
		return nil, fmt.Errorf("Can't advise you, all day is taken.")
	}
	return ret[0], nil
}

func (e *Entries) getTimeTable(prof iface.Id) (evenday.TimeTable, error) {
	ttFilter, err := e.db.NewFilter(e.timeTableColl, nil)
	empty := evenday.TimeTable{}
	if err != nil {
		return empty, err
	}
	timeTableQ := map[string]interface{}{
		"createdBy": prof,
	}
	ttFilter.AddQuery(timeTableQ)
	ttC, err := ttFilter.Count()
	if err != nil {
		return empty, err
	}
	if ttC != 1 {
		return empty, fmt.Errorf("Number of timeTables is not one.")
	}
	timeTables, err := ttFilter.Find()
	if err != nil {
		return empty, err
	}
	timeTable, err := evenday.GenericToTimeTable(timeTables[0]["timeTable"].([]interface{}))
	if err != nil {
		return empty, err
	}
	return timeTable, nil
}

// Checks if the timeTable is ok and the interval fits into the timeTable.
func (e *Entries) okAccordingToTimeTable(data map[string]interface{}, from, to int64) error {
	dayN := evenday.DateToDayName(from)
	prof, err := e.db.ToId(data["professional"].(string))
	if err != nil {
		return err
	}
	interval, err := evenday.GenericToInterval(from, to)
	if err != nil {
		return err
	}
	timeTable, err := e.getTimeTable(prof)
	if err != nil {
		return err
	}
	if !evenday.InTimeTable(dayN, interval, timeTable) {
		return fmt.Errorf("Interval does not fit into timeTable.")
	}
	return nil
}

func (e *Entries) othersAlreadyTook(a iface.Filter, from, to int64) error {
	entryQ := map[string]interface{}{
		"$or": []interface{}{
			map[string]interface{}{
				"from": map[string]interface{}{
					"$gt": from,
					"$lt": to,
				},
			},
			map[string]interface{}{
				"to": map[string]interface{}{
					"$gt": from,
					"$lt": to,
				},
			},
		},
	}
	a.AddQuery(entryQ)
	eC, err := a.Count()
	if err != nil {
		return err
	}
	if eC > 0 {
		return fmt.Errorf("That time is already taken.")
	}
	return nil
}

func (e *Entries) intervalIsValid(data map[string]interface{}, prof iface.Id, length int64) error {
	q := map[string]interface{}{
		"professional": prof,
		"length": length,
	}
	iFilter, err := e.db.NewFilter("intervals", q)
	if err != nil {
		return err
	}
	c, err := iFilter.Count()
	if err != nil {
		return err
	}
	if c != 1 {
		return fmt.Errorf("Interval %v is not defined.", length)
	}
	return nil
}

func (e *Entries) Insert(a iface.Filter, data map[string]interface{}) error {
	e.getOptions(a.Subject())
	from := data["from"].(int64)
	length := data["length"].(int64)
	prof, err := e.db.ToId(data["professional"].(string))
	if err != nil {
		return err
	}
	err = e.intervalIsValid(data, prof, length)
	if err != nil {
		return err
	}
	to := from + length * 60
	err = e.okAccordingToTimeTable(data, from, to)
	if err != nil {
		return err
	}
	err = e.othersAlreadyTook(a, from, to)
	if err != nil {
		return err
	}
	i := map[string]interface{}{
		"createdBy": e.userId,
		"from": from,
		"to": to,
		"length": length,
		"day": dateToString(from),
		"forProfessional": prof,
	}
	return a.Insert(i)
}

func (e *Entries) Delete(a iface.Filter) error {
	return nil
}

func (e *Entries) Install(o iface.Document, resource string) error {
	upd := map[string]interface{}{
		"$addToSet": map[string]interface{}{
			"Hooks." + resource + "TopModFilter": []interface{}{"meeting.entries", "TopModFilter"},
		},
	}
	return o.Update(upd)
}

func (e *Entries) Uninstall(o iface.Document, resource string) error {
	upd := map[string]interface{}{
		"$pull": map[string]interface{}{
			"Hooks." + resource + "TopModFilter": []interface{}{"meeting.entries", "TopModFilter"},
		},
	}
	return o.Update(upd)
}

type TimeTable struct {
	shared
}

func (tt *TimeTable) Init(ctx iface.Context) {
	tt.db = ctx.Db()
	tt.userId = ctx.User().Id()
	tt.userIsProfessional = isProfessional(ctx.User())
	tt.optDoc = ctx.Options().Document()
	tt.timeTableColl = "timeTables"
	tt.intervalColl = "intervals"
}

func toSS(sl []interface{}) []string {
	ret := []string{}
	for _, v := range sl {
		ret = append(ret, v.(string))
	}
	return ret
}

func (tt *TimeTable) Save(a iface.Filter, data map[string]interface{}) error {
	if !tt.userIsProfessional {
		return fmt.Errorf("Only professionals can save timetables.")
	}
	ssl := toSS(data["timeTable"].([]interface{}))
	timeTable, err := evenday.StringsToTimeTable(ssl)
	if err != nil {
		return err
	}
	count, err := a.Count()
	if err != nil {
		return err
	}
	m := map[string]interface{}{}
	m["createdBy"] = tt.userId
	m["timeTable"] = timeTable
	if count == 0 {
		return a.Insert(m)
	} else if count == 1 {
		doc, err := a.SelectOne()
		if err != nil {
			return err
		}
		return doc.Update(m)
	}
	_, err = a.RemoveAll()
	if err != nil {
		return err
	}
	return a.Insert(m)
}

func (tt *TimeTable) TopModFilter(a iface.Filter) {
	if tt.userIsProfessional {
		a.AddQuery(map[string]interface{}{
			"createdBy": tt.userId,
		})
	} else {
		a.AddQuery(map[string]interface{}{
			// Hehe.
			"clientsShouldNotViewTimeTables": true,
		})
	}
}

func (tt *TimeTable) AddTemplateBuiltin(b map[string]interface{}) {
	b["toTimeTable"] = evenday.GenericToTimeTable
}

func (tt *TimeTable) Install(o iface.Document, resource string) error {
	upd := map[string]interface{}{
		"$addToSet": map[string]interface{}{
			"Hooks." + resource + "TopModFilter": []interface{}{"meeting.timeTable", "TopModFilter"},
			"Hooks.AddTemplateBuiltin": []interface{}{"meeting.timeTable"},
		},
		"$set": map[string]interface{}{
			"nouns." + resource + ".verbs.Save.input": map[string]interface{}{
				"timeTable": map[string]interface{}{
					"type": "any",
					"slice": true,
					"must": true,
				},
			},
		},
	}
	return o.Update(upd)
}

func (tt *TimeTable) Uninstall(o iface.Document, resource string) error {
	upd := map[string]interface{}{
		"$pull": map[string]interface{}{
			"Hooks." + resource + "TopModFilter": []interface{}{"meeting.timeTable", "TopModFilter"},
			"Hooks.AddTemplateBuiltin": []interface{}{"meeting.timeTable"},
		},
	}
	return o.Update(upd)
}