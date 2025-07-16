package database

// Link represents a FTN node configuration and routing information
type Link struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StationName string `gorm:"column:station_name;not null" json:"station_name"`
	FtnAddress  string `gorm:"column:ftn_address;unique;not null" json:"ftn_address"`
	PktPassword string `gorm:"column:pkt_password;default:''" json:"pkt_password"`
	Password    string `gorm:"column:password;default:'-'" json:"password"`
	Address     string `gorm:"column:address;default:'-'" json:"address"`
}

func (Link) TableName() string {
	return "links"
}

// Echoarea represents message area definitions
type Echoarea struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"column:name;unique;not null" json:"name"`
	Description string `gorm:"column:description;size:1000" json:"description"`
	WLevel      int64  `gorm:"column:wlevel;default:0" json:"wlevel"`
	RLevel      int64  `gorm:"column:rlevel;default:0" json:"rlevel"`
	Grp         string `gorm:"column:grp;default:''" json:"grp"`
}

func (Echoarea) TableName() string {
	return "echoarea"
}

// Echomail represents public message storage
type Echomail struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EchoareaID  int64  `gorm:"column:echoarea_id;not null;index" json:"echoarea_id"`
	FromName    string `gorm:"column:from_name;not null" json:"from_name"`
	ToName      string `gorm:"column:to_name;not null" json:"to_name"`
	FromFtnAddr string `gorm:"column:from_ftn_addr;not null" json:"from_ftn_addr"`
	Date        int64  `gorm:"column:date" json:"date"`
	Subject     string `gorm:"column:subject;type:text" json:"subject"`
	Message     string `gorm:"column:message;type:text" json:"message"`
	SeenBy      string `gorm:"column:seen_by;type:text" json:"seen_by"`
	Path        string `gorm:"column:path;type:text" json:"path"`
	MsgID       string `gorm:"column:msgid;index" json:"msgid"`

	// Relationship
	Echoarea Echoarea `gorm:"foreignKey:EchoareaID;references:ID" json:"-"`
}

func (Echomail) TableName() string {
	return "echomail"
}

// Netmail represents private message storage
type Netmail struct {
	ID           int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FromName     string `gorm:"column:from_name" json:"from_name"`
	ToName       string `gorm:"column:to_name" json:"to_name"`
	FromAddress  string `gorm:"column:from_address;not null" json:"from_address"`
	ToAddress    string `gorm:"column:to_address;not null" json:"to_address"`
	Subject      string `gorm:"column:subject" json:"subject"`
	Text         string `gorm:"column:text;type:text" json:"text"`
	Date         int64  `gorm:"column:date" json:"date"`
	RouteVia     *int64 `gorm:"column:route_via;index" json:"route_via"`
	Send         bool   `gorm:"column:send;default:false;index" json:"send"`
	Attr         int    `gorm:"column:attr;default:256" json:"attr"`
	LastModified int64  `gorm:"column:last_modified" json:"last_modified"`

	// Relationship
	RouteLink *Link `gorm:"foreignKey:RouteVia;references:ID" json:"-"`
}

func (Netmail) TableName() string {
	return "netmail"
}

// Subscription represents echoarea subscriptions per link
type Subscription struct {
	LinkID     int64 `gorm:"column:link_id;primaryKey" json:"link_id"`
	EchoareaID int64 `gorm:"column:echoarea_id;primaryKey" json:"echoarea_id"`

	// Relationships
	Link     Link     `gorm:"foreignKey:LinkID;references:ID" json:"-"`
	Echoarea Echoarea `gorm:"foreignKey:EchoareaID;references:ID" json:"-"`
}

func (Subscription) TableName() string {
	return "subscription"
}

// EchomailAwaiting represents outbound echomail per link
type EchomailAwaiting struct {
	LinkID     int64 `gorm:"column:link_id;primaryKey" json:"link_id"`
	EchomailID int64 `gorm:"column:echomail_id;primaryKey" json:"echomail_id"`

	// Relationships
	Link     Link     `gorm:"foreignKey:LinkID;references:ID" json:"-"`
	Echomail Echomail `gorm:"foreignKey:EchomailID;references:ID" json:"-"`
}

func (EchomailAwaiting) TableName() string {
	return "echomailawait"
}

// Filearea represents file distribution areas
type Filearea struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"column:name;unique;not null" json:"name"`
	Description string `gorm:"column:description;size:1000" json:"description"`
	WLevel      int64  `gorm:"column:wlevel;default:0" json:"wlevel"`
	RLevel      int64  `gorm:"column:rlevel;default:0" json:"rlevel"`
	Grp         string `gorm:"column:grp;default:''" json:"grp"`
}

func (Filearea) TableName() string {
	return "filearea"
}

// Filemail represents file distribution entries
type Filemail struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FilearaID int64  `gorm:"column:filearea_id;not null;index" json:"filearea_id"`
	Filename  string `gorm:"column:filename;type:text" json:"filename"`
	Filedesc  string `gorm:"column:filedesc;type:text" json:"filedesc"`
	Filepath  string `gorm:"column:filepath" json:"filepath"`
	Origin    string `gorm:"column:origin" json:"origin"`
	SeenBy    string `gorm:"column:seenby;type:text" json:"seenby"`
	Path      string `gorm:"column:path;type:text" json:"path"`
	Created   int64  `gorm:"column:created" json:"created"`

	// Relationship
	Filearea Filearea `gorm:"foreignKey:FilearaID;references:ID" json:"-"`
}

func (Filemail) TableName() string {
	return "filemail"
}

// FileSubscription represents file area subscriptions per link
type FileSubscription struct {
	LinkID    int64 `gorm:"column:link_id;primaryKey" json:"link_id"`
	FilearaID int64 `gorm:"column:filearea_id;primaryKey" json:"filearea_id"`

	// Relationships
	Link     Link     `gorm:"foreignKey:LinkID;references:ID" json:"-"`
	Filearea Filearea `gorm:"foreignKey:FilearaID;references:ID" json:"-"`
}

func (FileSubscription) TableName() string {
	return "filesubscription"
}

// FilemailAwaiting represents outbound files per link
type FilemailAwaiting struct {
	LinkID     int64 `gorm:"column:link_id;primaryKey" json:"link_id"`
	FilemailID int64 `gorm:"column:filemail_id;primaryKey" json:"filemail_id"`

	// Relationships
	Link     Link     `gorm:"foreignKey:LinkID;references:ID" json:"-"`
	Filemail Filemail `gorm:"foreignKey:FilemailID;references:ID" json:"-"`
}

func (FilemailAwaiting) TableName() string {
	return "filemailawaiting"
}

// LinkOption represents link-specific configuration options
type LinkOption struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LinkID int64  `gorm:"column:link_id;not null;uniqueIndex:idx_link_option" json:"link_id"`
	Name   string `gorm:"column:name;not null;uniqueIndex:idx_link_option" json:"name"`
	Value  string `gorm:"column:value;type:text;not null" json:"value"`

	// Relationship
	Link Link `gorm:"foreignKey:LinkID;references:ID" json:"-"`
}

func (LinkOption) TableName() string {
	return "linkoptions"
}

// Route represents netmail routing rules
type Route struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Nice        int64  `gorm:"column:nice" json:"nice"`
	FromName    string `gorm:"column:from_name;default:'*'" json:"from_name"`
	ToName      string `gorm:"column:to_name;default:'*'" json:"to_name"`
	FromAddress string `gorm:"column:from_address;default:'*'" json:"from_address"`
	ToAddress   string `gorm:"column:to_address;default:'*'" json:"to_address"`
	Subject     string `gorm:"column:subject;default:'*'" json:"subject"`
	RouteVia    int64  `gorm:"column:route_via;not null" json:"route_via"`

	// Relationship
	RouteLink Link `gorm:"foreignKey:RouteVia;references:ID" json:"-"`
}

func (Route) TableName() string {
	return "routing"
}

// Jscript represents stored JavaScript code
type Jscript struct {
	ID      int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Content string `gorm:"column:content;type:text" json:"content"`
}

func (Jscript) TableName() string {
	return "jscripts"
}

// ScriptHelper represents JavaScript helper registration
type ScriptHelper struct {
	Helper    string `gorm:"column:helper;primaryKey" json:"helper"`
	ClassName string `gorm:"column:className;not null" json:"className"`
}

func (ScriptHelper) TableName() string {
	return "scripthelpers"
}

// Schedule represents script execution scheduling
type Schedule struct {
	ID          int64        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Type        ScheduleType `gorm:"column:type;not null" json:"type"`
	Details     int          `gorm:"column:details;default:0" json:"details"`
	JscriptID   int64        `gorm:"column:jscript_id;not null;unique" json:"jscript_id"`
	LastRunDate *int64       `gorm:"column:lastRunDate" json:"lastRunDate"`

	// Relationship
	Jscript Jscript `gorm:"foreignKey:JscriptID;references:ID" json:"-"`
}

func (Schedule) TableName() string {
	return "schedule"
}

// Robot represents external robot registration
type Robot struct {
	Robot     string `gorm:"column:robot;primaryKey" json:"robot"`
	ClassName string `gorm:"column:className;not null" json:"className"`
}

func (Robot) TableName() string {
	return "robots"
}

// NetmailAwaiting represents outbound netmail routing queue
type NetmailAwaiting struct {
	LinkID    int64 `gorm:"column:link_id;primaryKey" json:"link_id"`
	NetmailID int64 `gorm:"column:netmail_id;primaryKey" json:"netmail_id"`

	// Relationships
	Link    Link    `gorm:"foreignKey:LinkID;references:ID" json:"-"`
	Netmail Netmail `gorm:"foreignKey:NetmailID;references:ID" json:"-"`
}

func (NetmailAwaiting) TableName() string {
	return "netmailawaiting"
}
