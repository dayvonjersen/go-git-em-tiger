package main

func splitArgs(input string) (args []string) {
	var (
		in   = []rune(input)
		arg  = ""
		quot rune
		esc  bool
	)

	for i, r := range in {
		switch r {
		case '\\':
			if esc || i == len(in)-1 {
				arg += string(r)
			}
			esc = !esc
		case '\'', '"':
			if (quot == 0 || quot == r) && !esc {
				if quot == 0 {
					quot = r
				} else {
					quot = 0
				}
			} else {
				arg += string(r)
				esc = false
			}
		case ' ':
			if quot != 0 {
				arg += string(r)
			} else if len(arg) > 0 {
				args = append(args, arg)
				arg = ""
			}
		default:
			arg += string(r)
			esc = false
		}
	}

	args = append(args, arg)

	return
}
