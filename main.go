package main

//----- Packages -----
import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"

	"strconv"
	"time"

	apiLib "github.com/hornbill/goApiLib"
	"github.com/hornbill/pb"
)

//----- Main Function -----
func main() {

	//-- Initiate Variables
	initVars()

	//-- Process Flags
	procFlags()

	//-- If configVersion just output version number and die
	if configVersion {
		fmt.Printf("%v \n", version)
		return
	}

	//-- Load Configuration File Into Struct
	SQLImportConf = loadConfig()

	//-- Validation on Configuration File
	err := validateConf()
	if err != nil {
		logger(4, fmt.Sprintf("%v", err), true)
		logger(4, "Please Check your Configuration File: "+configFileName, true)
		return
	}

	logger(1, "Instance ID: "+SQLImportConf.InstanceID, true)
	//-- Once we have loaded the config write to hornbill log file
	logged := espLogger("---- XMLMC SQL Import Utility V"+fmt.Sprintf("%v", version)+" ----", "debug")

	if !logged {
		logger(4, "Unable to Connect to Instance", true)
		return
	}

	//Set SWSQLDriver to mysql320
	if SQLImportConf.SQLConf.Driver == "swsql" {
		SQLImportConf.SQLConf.Driver = "mysql320"
	}

	boolSQLOrgs, arrOrgs := queryDatabase()
	if boolSQLOrgs {
		processOrgs(arrOrgs)
	} else {
		logger(4, "No Results found", true)
		return
	}

	outputEnd()
}

func processOrgs(arrOrgs []map[string]interface{}) {
	bar := pb.StartNew(len(arrOrgs))
	logger(1, "Processing Organisations...\n", false)

	//Get the identity of the organisation name field from the config
	OrgNameField := fmt.Sprintf("%v", SQLImportConf.SQLConf.OrganizationName)

	espXmlmc = apiLib.NewXmlmcInstance(SQLImportConf.InstanceID)
	espXmlmc.SetAPIKey(SQLImportConf.APIKey)

	//-- Loop each organisation
	for _, orgRecord := range arrOrgs {
		//Get the organisation name for the current record
		orgName := ""
		if orgRecord[OrgNameField] != nil {
			orgName = fmt.Sprintf("%s", orgRecord[OrgNameField])
		}
		if orgName != "" {
			var buffer bytes.Buffer
			buffer.WriteString("[DEBUG] Processing Organisation [" + orgName + "]\n")
			foundID, foundName, err := checkOrgOnInstance(orgName, espXmlmc, &buffer)
			if err == nil {
				if foundID > 0 && (SQLImportConf.OrganisationAction == "Update" || SQLImportConf.OrganisationAction == "Both") {
					buffer.WriteString(loggerGen(1, "Update Organisation: ["+orgName+"] ["+strconv.Itoa(foundID)+"]"))
					upsertOrg(orgRecord, espXmlmc, orgName, foundID, foundName, &buffer)
				} else if foundID <= 0 && (SQLImportConf.OrganisationAction == "Create" || SQLImportConf.OrganisationAction == "Both") {
					buffer.WriteString(loggerGen(1, "Create Organisation: ["+orgName+"]"))
					upsertOrg(orgRecord, espXmlmc, orgName, foundID, foundName, &buffer)
				} else if foundID > 0 && SQLImportConf.OrganisationAction == "Create" {
					buffer.WriteString(loggerGen(1, "Configuration set to create only but organisation already exists"))
				}
			}
			loggerWriteBuffer(buffer.String())
			buffer.Reset()
		} else {
			logger(4, "Org Name not found. Check your configuration file or data source", false)
		}
		bar.Increment()
	}
	bar.FinishPrint("Processing Complete!")
}

func checkOrgOnInstance(oName string, espXmlmc *apiLib.XmlmcInstStruct, buffer *bytes.Buffer) (int, string, error) {
	intReturn := -1
	strReturn := ""
	espXmlmc.SetParam("entity", "Organizations")
	espXmlmc.SetParam("matchScope", "all")
	espXmlmc.OpenElement("searchFilter")
	espXmlmc.SetParam("column", "h_organization_name")
	espXmlmc.SetParam("value", oName)
	espXmlmc.CloseElement("searchFilter")
	espXmlmc.SetParam("maxResults", "1")
	XMLCheckOrg, xmlmcErr := espXmlmc.Invoke("data", "entityBrowseRecords2")

	var xmlResponse xmlmcOrganisationSearchResponse
	if xmlmcErr != nil {
		buffer.WriteString(loggerGen(3, "Search for Organisation Unsuccessful. API Invoke Error from [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", xmlmcErr)))
		return 0, "", xmlmcErr
	}
	err := xml.Unmarshal([]byte(XMLCheckOrg), &xmlResponse)
	if err != nil {
		buffer.WriteString(loggerGen(3, "Search for Organisation Unsuccessful. Unmarshall Error from [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", err)))
	} else {
		if xmlResponse.MethodResult != constOK {
			err = errors.New(xmlResponse.State.ErrorRet)
			buffer.WriteString(loggerGen(3, "Search for Organisation Unsuccessful. MethodResult not OK from [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", err)))
		} else {
			//-- Check Response
			if len(xmlResponse.Orgs) > 0 && xmlResponse.Orgs[0].OrganizationID != 0 {
				intReturn = xmlResponse.Orgs[0].OrganizationID
				strReturn = xmlResponse.Orgs[0].OrganizationName
				if err != nil {
					buffer.WriteString(loggerGen(3, "Search for Organisation Unsuccessful. Key Type Conversion Failed [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", err)))
					intReturn = -1
				}
			} else {
				if fmt.Sprintf("%v", err) == "<nil>" {
					buffer.WriteString(loggerGen(1, "Organisation not returned from API call [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", err)))
				} else {
					buffer.WriteString(loggerGen(3, "Organisation not returned from API call [entityBrowseRecords2] [Organisation]: "+fmt.Sprintf("%v", err)))
				}

			}
		}
	}
	return intReturn, strReturn, err
}

func upsertOrg(u map[string]interface{}, espXmlmc *apiLib.XmlmcInstStruct, oName string, foundID int, foundName string, buffer *bytes.Buffer) {
	insertOrg := (foundID <= 0)
	p := make(map[string]string)
	for key, value := range u {
		if value == nil {
			value = ""
		}
		p[key] = fmt.Sprintf("%s", value)
	}
	espXmlmc.SetParam("entity", "Organizations")
	espXmlmc.SetParam("returnModifiedData", "true")
	espXmlmc.OpenElement("primaryEntityData")
	espXmlmc.OpenElement("record")

	if !insertOrg {
		espXmlmc.SetParam("h_organization_id", fmt.Sprintf("%d", foundID))
	}

	for hbCol, dbCol := range SQLImportConf.OrganizationMapping {
		espXmlmc.SetParam("h_"+hbCol, p[dbCol])
	}
	espXmlmc.CloseElement("record")
	espXmlmc.CloseElement("primaryEntityData")

	if !insertOrg && oName != foundName {
		espXmlmc.OpenElement("relatedEntityData")
		espXmlmc.SetParam("relationshipName", "Container")
		espXmlmc.SetParam("entityAction", "update")
		espXmlmc.OpenElement("record")
		espXmlmc.SetParam("h_name", oName)
		espXmlmc.CloseElement("record")
		espXmlmc.CloseElement("relatedEntityData")
	}

	var XMLSTRING = espXmlmc.GetParam()
	logger(1, "Organisation Create/Update XML: "+XMLSTRING, false)

	//-- Check for Dry Run
	if !configDryRun {
		var XMLCreate string
		var xmlmcErr error
		method := "entityAddRecord"
		if !insertOrg {
			method = "entityUpdateRecord"
		}
		XMLCreate, xmlmcErr = espXmlmc.Invoke("data", method)
		var xmlResponse xmlmcPrimEntResponse
		if xmlmcErr != nil {
			counters.errorCount++
			buffer.WriteString(loggerGen(4, "Organisation Create/Update Failed. API Invoke Error from ["+method+"] : "+fmt.Sprintf("%v", xmlmcErr)))
			return
		}
		err := xml.Unmarshal([]byte(XMLCreate), &xmlResponse)
		if err != nil {
			counters.errorCount++
			buffer.WriteString(loggerGen(4, "Organisation Create/Update Failed. Unmarshall Error from ["+method+"] : "+fmt.Sprintf("%v", err)))
			return
		}
		if xmlResponse.MethodResult != constOK {
			err = errors.New(xmlResponse.State.ErrorRet)
			if fmt.Sprintf("%v", err) == "There are no values to update" {
				buffer.WriteString(loggerGen(1, "MethodResult Not OK for Organisation Create/Update Failed from ["+method+"] : "+fmt.Sprintf("%v", err)))
			} else {
				counters.errorCount++
				buffer.WriteString(loggerGen(4, "Organisation Create/Update Failed. MethodResult Not OK from ["+method+"] : "+fmt.Sprintf("%v", err)))
			}
			return
		}

		if insertOrg {
			buffer.WriteString(loggerGen(1, "Organisation Create Success"))
			counters.created++
		} else if len(xmlResponse.Record.ColList) > 0 {
			buffer.WriteString(loggerGen(1, "Organisation Update Success"))
			counters.updated++
		} else {
			buffer.WriteString(loggerGen(1, "Organisation Update: Nothing to update"))
		}

		if insertOrg {
			var xmlRelationResponse xmlmcResponse
			espXmlmc.ClearParam()
			espXmlmc.SetParam("entity", "container")
			espXmlmc.SetParam("returnModifiedData", "true")
			espXmlmc.OpenElement("primaryEntityData")
			espXmlmc.OpenElement("record")
			espXmlmc.SetParam("h_name", oName)
			espXmlmc.SetParam("h_type", "Organizations")
			espXmlmc.SetParam("h_type_id", strconv.Itoa(xmlResponse.Record.OrgID))
			espXmlmc.SetParam("h_is_root", "1")
			espXmlmc.CloseElement("record")
			espXmlmc.CloseElement("primaryEntityData")

			var XMLSTRING = espXmlmc.GetParam()
			logger(1, "Container Create XML: "+XMLSTRING, false)

			XMLCreate, xmlmcErr = espXmlmc.Invoke("data", "entityAddRecord")
			if xmlmcErr != nil {
				counters.errorCount++
				buffer.WriteString(loggerGen(3, "Adding Organisation to Container Unsuccessful. API Invoke Error from [entityAddRecord] for entity [RelatedContainer]: "+fmt.Sprintf("%v", xmlmcErr)))
				return
			}
			err := xml.Unmarshal([]byte(XMLCreate), &xmlRelationResponse)
			if err != nil {
				counters.errorCount++
				buffer.WriteString(loggerGen(3, "Adding Organisation to Container Unsuccessful. Unmarshall Error from [entityAddRecord] for entity [RelatedContainer]: "+fmt.Sprintf("%v", err)))
				return
			}
			buffer.WriteString(loggerGen(1, "Adding Organisation to Container Success"))
		}
	} else {
		espXmlmc.ClearParam()
	}
}

func outputEnd() {
	//-- End output
	if counters.errorCount > 0 {
		logger(4, "Error encountered please check the log file", true)
		logger(4, "Error Count: "+fmt.Sprintf("%d", counters.errorCount), true)
	} else {
		logger(1, "No errors encountered", true)
	}
	logger(1, "Updated: "+fmt.Sprintf("%d", counters.updated), true)
	logger(1, "Created: "+fmt.Sprintf("%d", counters.created), true)

	//-- Show Time Takens
	endTime = time.Since(startTime)
	logger(1, "Time Taken: "+fmt.Sprintf("%v", endTime), true)

	//-- End output to log
	espLogger("Errors: "+fmt.Sprintf("%d", counters.errorCount), "error")
	espLogger("Updated: "+fmt.Sprintf("%d", counters.updated), "debug")
	espLogger("Created: "+fmt.Sprintf("%d", counters.created), "debug")
	espLogger("Time Taken: "+fmt.Sprintf("%v", endTime), "debug")
	espLogger("---- XMLMC SQL Org Import Complete ---- ", "debug")
	logger(1, "---- XMLMC SQL Org Import Complete ---- ", true)
}
