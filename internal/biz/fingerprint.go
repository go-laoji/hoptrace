package biz

import (
	"regexp"
	"strings"
)

// Fingerprint 设备指纹识别结果。
type Fingerprint struct {
	Vendor    string
	Family    string
	Model     string
	OSVersion string
}

// 企业号 → 厂商
var enterpriseVendors = map[string]string{
	"2011":  "Huawei",
	"25506": "H3C",
	"4881":  "Ruijie",
	"9":     "Cisco",
	"2636":  "Juniper",
	"14988": "MikroTik",
	"3902":  "ZTE",
	"1916":  "Extreme",
	"11":    "HP",
	"674":   "Dell",
	"12356": "Fortinet",
	"25461": "PaloAlto",
}

var (
	// 例:Huawei Versatile Routing Platform Software VRP (R) software, Version 5.170 (S5700-28C-HI V200R011C10SPC600)
	huaweiModelVer = regexp.MustCompile(`\(([A-Za-z][A-Za-z0-9\-/]*)\s+(V\d{3}R\d{3}C\d{2}[A-Za-z0-9]*)\)`)
	huaweiModelTok = regexp.MustCompile(`(?i)\b(S\d{4}[A-Z0-9\-]*|CE\d{4,5}[A-Z0-9\-]*|USG[0-9A-Z\-]+|HiSecEngine[0-9A-Z\-]*|Eudemon[0-9A-Z\-]*|AR\d{3,4}[A-Z0-9\-]*|NE\d+[A-Z0-9\-]*|AC\d{4}[A-Z0-9\-]*|AirEngine[0-9A-Z\-]*)\b`)
)

// Identify 根据 sysObjectID 与 sysDescr 识别厂商/家族/型号/版本。
// 华为的 sysObjectID 前缀为 1.3.6.1.4.1.2011;具体型号子树需结合真机校准,这里以 sysDescr 为主。
func Identify(sysOID, sysDescr string) Fingerprint {
	fp := Fingerprint{Vendor: "Unknown", Family: FamilyUnknown}
	oid := strings.TrimPrefix(sysOID, ".")
	const entPrefix = "1.3.6.1.4.1."
	if strings.HasPrefix(oid, entPrefix) {
		ent := strings.SplitN(strings.TrimPrefix(oid, entPrefix), ".", 2)[0]
		if v, ok := enterpriseVendors[ent]; ok {
			fp.Vendor = v
		}
	}
	descr := sysDescr
	lower := strings.ToLower(descr)
	if fp.Vendor == "Unknown" && strings.Contains(lower, "huawei") {
		fp.Vendor = "Huawei"
	}

	if fp.Vendor == "Huawei" {
		if m := huaweiModelVer.FindStringSubmatch(descr); m != nil {
			fp.Model, fp.OSVersion = m[1], m[2]
		} else if m := huaweiModelTok.FindString(descr); m != "" {
			fp.Model = m
		}
		fp.Family = huaweiFamily(fp.Model, lower)
		return fp
	}

	switch {
	case strings.Contains(lower, "firewall") || strings.Contains(lower, "fortigate") || strings.Contains(lower, "asa"):
		fp.Family = FamilyFirewall
	case strings.Contains(lower, "switch"):
		fp.Family = FamilySwitch
	case strings.Contains(lower, "router"):
		fp.Family = FamilyRouter
	}
	return fp
}

func huaweiFamily(model, lowerDescr string) string {
	m := strings.ToUpper(model)
	switch {
	case strings.HasPrefix(m, "USG"), strings.HasPrefix(m, "EUDEMON"), strings.HasPrefix(m, "HISECENGINE"):
		return FamilyFirewall
	case strings.HasPrefix(m, "AR"), strings.HasPrefix(m, "NE"):
		return FamilyRouter
	case strings.HasPrefix(m, "AIRENGINE"), regexp.MustCompile(`^AC\d`).MatchString(m):
		return FamilyWLANAC
	case strings.HasPrefix(m, "S"), strings.HasPrefix(m, "CE"):
		return FamilySwitch
	}
	switch {
	case strings.Contains(lowerDescr, "usg") || strings.Contains(lowerDescr, "firewall"):
		return FamilyFirewall
	case strings.Contains(lowerDescr, "switch"):
		return FamilySwitch
	case strings.Contains(lowerDescr, "router"):
		return FamilyRouter
	}
	return FamilyUnknown
}

// RoleByFamily 由设备家族给出初始角色;交换机的 core/aggregation/access 由拓扑位置进一步判定。
func RoleByFamily(family string) string {
	switch family {
	case FamilyFirewall:
		return RoleFirewall
	case FamilyRouter:
		return RoleRouter
	case FamilyWLANAC:
		return RoleAC
	case FamilySwitch:
		return RoleAggregation
	}
	return RoleUnknown
}
