package auth

import (
	"fmt"
)

type Article struct {
	ID               int
	Title            string
	Tag              string
	TagID            int
	Author           string
	AuthorID         int
	OriginalAuthorID int
	OriginalAuthor   string
	Content          string
	Raw              string
	Timestamp        int
	ModTimestamp     int
	Deleted          bool
	Locked           bool
	Read             bool
	Hits             int
	ParentID         int
	ParentTitle      string
	Children         int
	Revision         int

	IsRestricted     bool
	IsOthersMessage  bool
	IsMessage        bool
	IsMessageSentout bool
}

type Message struct {
	ID           int
	Title        string
	Preview      string
	ReceiverID   int
	ReceiverName string
	SenderID     int
	SenderName   string
	Sentout      bool
	Timestamp    int
	Read         bool
}

type Image struct {
	ID         int
	UploaderID int
	Uploader   string
	Path       string
	ThumbPath  string
	Filename   string
	ShortName  string
	Timestamp  int
	Hits       int
	Hide       bool
	R18        bool
	Size       int
}

type AuthUser struct {
	ID             int
	Name           string
	NickName       string
	LastLoginDate  int
	SignUpDate     int
	LastLoginIP    string
	Status         string
	Group          string
	Avatar         string
	AvatarThumb    string
	ImageUsage     int
	SessionID      string
	GalleryVisible string

	Index   Article
	IndexID int
}

type SubTag struct {
	ID          int
	Name        string
	Alias       string
	Description string
	Children    int
}

type BackForth struct {
	NextPage string
	PrevPage string

	LastDayPage   string
	LastWeekPage  string
	LastMonthPage string
	LastYearPage  string

	NextDayPage   string
	NextWeekPage  string
	NextMonthPage string
	NextYearPage  string

	Range struct {
		Start int
		End   int
	}
}

func (bf *BackForth) Set(prev, next int) {
	make1 := func(t int) string {
		return fmt.Sprintf("before=%s_%s", HashTS(t), To60(uint64(t)))
	}

	make2 := func(t int) string {
		return fmt.Sprintf("after=%s_%s", HashTS(t), To60(uint64(t)))
	}

	bf.PrevPage = make2(prev)
	bf.NextPage = make1(next)

	bf.LastDayPage = make1(prev - 3600000*24)
	bf.LastWeekPage = make1(prev - 3600000*24*7)
	bf.LastMonthPage = make1(prev - 3600000*24*30)
	bf.LastYearPage = make1(prev - 3600000*24*365)

	bf.NextDayPage = make2(prev + 3600000*24)
	bf.NextWeekPage = make2(prev + 3600000*24*7)
	bf.NextMonthPage = make2(prev + 3600000*24*30)
	bf.NextYearPage = make2(prev + 3600000*24*365)

	bf.Range.Start = next
	bf.Range.End = prev
}
