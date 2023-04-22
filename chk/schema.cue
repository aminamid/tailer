package tailer

#MoniterFile: {
  "id": string & !="",
  "path": string & !="",
  "type": *"mxlog" | "mxstat" | "rglog" | "mosstat" | "moslog", 
}

files: [...#MoniterFile]
