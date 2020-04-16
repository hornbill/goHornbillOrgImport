package main

//----- Packages -----
import (
	"time"

	apiLib "github.com/hornbill/goApiLib"
)

//----- Constants -----
const (
	version = "1.0.2"
	constOK = "ok"
)

//----- Variables -----
var (
	SQLImportConf   SQLImportConfStruct
	counters        counterTypeStruct
	configFileName  string
	configLogPrefix string
	configDryRun    bool
	configVersion   bool
	configDebug     bool
	timeNow         string
	startTime       time.Time
	endTime         time.Duration
	espXmlmc        *apiLib.XmlmcInstStruct
)

//----- Structs -----
type xmlmcOrganisationSearchResponse struct {
	MethodResult string                     `xml:"status,attr"`
	Orgs         []organisationObjectStruct `xml:"params>rowData>row"`
	State        stateStruct                `xml:"state"`
}

type organisationObjectStruct struct {
	OrganizationID   int    `xml:"h_organization_id"`
	OrganizationName string `xml:"h_organization_name"`
}

type counterTypeStruct struct {
	updated    uint16
	created    uint16
	errorCount uint64
}

//SQLImportConfStruct - Struct that defines the import config schema
type SQLImportConfStruct struct {
	APIKey              string
	InstanceID          string
	OrganisationAction  string
	SQLConf             sqlConfStruct
	OrganizationMapping map[string]string
}
type xmlmcResponse struct {
	MethodResult string      `xml:"status,attr"`
	State        stateStruct `xml:"state"`
}
type stateStruct struct {
	Code     string `xml:"code"`
	ErrorRet string `xml:"error"`
}

type sqlConfStruct struct {
	Driver           string
	Server           string
	UserName         string
	Password         string
	Port             int
	Query            string
	Database         string
	Encrypt          bool
	OrganizationName string
}
type xmlmcPrimEntResponse struct {
	MethodResult string                     `xml:"status,attr"`
	Record       xmlmcPrimEntResponseRecord `xml:"params>primaryEntityData>record"`
	State        stateStruct                `xml:"state"`
}

type xmlmcPrimEntResponseRecord struct {
	MethodResult string       `xml:"status,attr"`
	OrgID        int          `xml:"h_organization_id"`
	ColList      []recordCols `xml:",any"`
}

type recordCols struct {
	Amount string `xml:",chardata"`
}
