package main

// deviceNames maps factory model codes to consumer-friendly marketing names
var deviceNames = map[string]string{
	// Samsung Galaxy S Series
	"SM-G991B":   "Galaxy S21",
	"SM-G991U":   "Galaxy S21",
	"SM-G996B":   "Galaxy S21+",
	"SM-G998B":   "Galaxy S21 Ultra",
	"SM-S901B":   "Galaxy S22",
	"SM-S906B":   "Galaxy S22+",
	"SM-S908B":   "Galaxy S22 Ultra",
	"SM-S911B":   "Galaxy S23",
	"SM-S916B":   "Galaxy S23+",
	"SM-S918B":   "Galaxy S23 Ultra",
	"SM-S921B":   "Galaxy S24",
	"SM-S926B":   "Galaxy S24+",
	"SM-S928B":   "Galaxy S24 Ultra",
	
	// Samsung Galaxy A Series
	"SM-A125F":   "Galaxy A12",
	"SM-A135F":   "Galaxy A13",
	"SM-A225F":   "Galaxy A22 5G",
	"SM-A325F":   "Galaxy A32",
	"SM-A336B":   "Galaxy A33 5G",
	"SM-A525F":   "Galaxy A52",
	"SM-A526B":   "Galaxy A52s 5G",
	"SM-A536B":   "Galaxy A53 5G",
	"SM-A546B":   "Galaxy A54 5G",
	"SM-A715F":   "Galaxy A71",
	"SM-A725F":   "Galaxy A72",
	
	// Google Pixel
	"Pixel":      "Pixel",
	"Pixel 2":    "Pixel 2",
	"Pixel 3":    "Pixel 3",
	"Pixel 4":    "Pixel 4",
	"Pixel 5":    "Pixel 5",
	"Pixel 6":    "Pixel 6",
	"Pixel 6 Pro": "Pixel 6 Pro",
	"Pixel 7":    "Pixel 7",
	"Pixel 7 Pro": "Pixel 7 Pro",
	"Pixel 8":    "Pixel 8",
	"Pixel 8 Pro": "Pixel 8 Pro",
	
	// Xiaomi/Redmi
	"M2012K11G":  "Mi 11",
	"M2102J20SG": "Mi 11 Ultra",
	"2201123G":   "12T Pro",
	"2210132G":   "13",
	"23021RAA2G": "13 Pro",
	"2107113SG":  "Redmi Note 10",
	"21091116AG": "Redmi Note 10 Pro",
	"2201116SG":  "Redmi Note 11",
	"2201116PG":  "Redmi Note 11 Pro",
	"22101316G":  "Redmi Note 12",
	"23021RAAEG": "Redmi Note 12 Pro",
	
	// OnePlus
	"CPH2205":    "OnePlus 9",
	"CPH2213":    "OnePlus 9 Pro",
	"CPH2305":    "OnePlus 10 Pro",
	"CPH2415":    "OnePlus 11",
	"NE2213":     "OnePlus 10 Pro",
	"NE2215":     "OnePlus 11",
	
	// Motorola
	"XT2125-4":   "Moto G Power (2021)",
	"XT2117-4":   "Moto G Stylus (2021)",
	"XT2163-5":   "Edge 2022",

	// ASUS ROG Phone Series
	"ASUS_I001D": "ROG Phone 2",
	"ASUS_I003D": "ROG Phone 3",
	"ASUS_I005D": "ROG Phone 5",
	"ASUS_AI2201": "ROG Phone 6",
	"ASUS_AI2302": "ROG Phone 7",
	"ASUS_AI2401": "ROG Phone 8",
	"ZS600KL":    "ROG Phone",
	
	// ASUS Zenfone Series
	"ASUS_AI2203": "Zenfone 9",
	"ASUS_AI2301": "Zenfone 10",
	"ZS630KL":    "Zenfone 7",
	"ZS670KS":    "Zenfone 7 Pro",
	"ZS681KS":    "Zenfone 8",
	"ZS590KS":    "Zenfone 8 Flip",
	"ASUS_X00TD": "Zenfone Max Pro M1",
}