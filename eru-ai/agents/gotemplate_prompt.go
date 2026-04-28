package agents

const TemplateVarsSchemaString = `{"type":"object","properties":{"Headers":{"type":"object"},"FormData":{"type":"object"},"FileData":{"type":"object"},"Params":{"type":"object"},"Vars":{"type":"object","properties":{"Body":{"type":"object"},"OrgBody":{"type":"object"}},"required":[]},"Body":{"type":"object"},"OrgBody":{"type":"object"},"Token":{"type":"object"},"FormDataKeyArray":{"type":"array","items":[{"type":"string"}]},"LoopVars":{"type":"array","items":[{"type":"object"}]},"LoopVar":{"type":"object"},"Cookies":{"type":"object"},"ResponseStatus":{"type":"integer"}},"required":[]}`

const GoTemplateContextVariablePrompt = `

There are three attributes in the context variable :
1. Vars or vars : this is JSON object for the current function step and its type is of TemplateVars
2. ReqVars or req_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous REQUEST objects of previous function steps
3. ResVars or res_vars : this is map of string as key and JSON object of type TemplateVars as value. The map key is the name of previous function steps. This holds all the previous RESPONSE objects of previous function steps
TemplateVars JSON schema is as follows :

There are many custom functions written by us that we can use in the gotemplate :

JSON Functions:
1. stringify : this function takes a JSON object and returns a string representation of the JSON object
2. unquote : this function takes a string and returns the unquoted string
3. marshalJSON : marshals interface{} to JSON bytes
4. unmarshalJSON : unmarshals JSON bytes to interface{}

Encoding Functions:
5. b64Encode : base64 encode bytes to string
6. b64Decode : base64 decode string to string
7. hexEncode : hex encode bytes to string
8. hexDecode : hex decode string to string

Crypto Functions:
9. aesEncryptGCM : AES GCM encryption
10. aesDecryptGCM : AES GCM decryption
11. aesEncryptECB : AES ECB encryption
12. aesDecryptECB : AES ECB decryption
13. aesEncryptCBC : AES CBC encryption
14. aesDecryptCBC : AES CBC decryption
15. generateAesKey : generate AES key of specified bits
16. generate_rsa_keypair : generate RSA key pair
17. encryptRSACert : encrypt with RSA certificate
18. hmac : HMAC with secret
19. shaHash : SHA hash with specified bits (256, 512)
20. md5 : MD5 hash
21. PKCS7Pad : PKCS7 padding
22. PKCS7Unpad : PKCS7 unpadding
23. new_jwt : create JWT token

String/Data Functions:
24. bytesToString : convert bytes to string
25. stringToByte : convert string to bytes
26. doubleQuote : double quote a string
27. len : get length of data
28. str_concat : concatenate strings with separator
29. str_replace : replace string occurrences
30. removenull : remove null characters
31. char_index : find character index in string

Map/Array Functions:
32. saveVar : save variable to map
33. concatMapKeyVal : concatenate map key-value pairs
34. concatMapKeyValUnordered : concatenate map unordered
35. makeMapKeyValUnordered : create map from string
36. overwriteMap : overwrite map with new data
37. removeMapKey : remove key from map
38. getMapValue : get value from map by key
39. getMapKeys : get all keys from map
40. getMapPointerValue : get pointer value from map
41. getArrayValue : get array value by index
42. arrayLen : get array length
43. is_array : check if variable is array
44. sortMapArray : sort array of maps by key

Math Functions:
45. math_add : add multiple numbers
46. math_sub : subtract two numbers
47. math_div : divide two numbers
48. math_mul : multiply two numbers
49. math_round : round number with precision

Date/Time Functions:
50. current_date : get current date
51. date_diff : add/subtract days/months/years from date
52. date_part : extract part from date (DAY, MONTH, YEAR)
53. date_format : format date from one layout to another

Utility Functions:
54. uuid : generate UUID
55. null : return null value
56. inc : increment number by 1
57. logobject : log object to console
58. logstring : log string to console
59. logerror : log error to console

Template/Filter Functions:
60. evalFilter : evaluate filter against record
61. makeFilter : create SQL filter from JSON
62. makeParentFilter : create parent SQL filter
63. fetch_filter_keys : fetch filter keys
64. execTemplate : execute sub-template

File/Data Processing:
65. excelToJson : convert Excel data to JSON
66. jsonToCsv : convert JSON to CSV string
67. jsonToCsvB64 : convert JSON to base64 CSV
68. kmsDecrypt : decrypt using KMS
69. getObjDiff : get difference between objects

All Sprig template functions are also available (date, string, math, flow control functions)
`
