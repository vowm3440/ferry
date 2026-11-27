package rulesets

import "regexp/syntax"

// Regex source bytes do not bound Thompson-program size: counted repetition
// can expand tiny input dramatically. Budget before Simplify/Compile and
// share the remaining budget across all regular expressions in a rule set.
func regexCost(r *syntax.Regexp, limit int) int {
	switch r.Op {
	case syntax.OpLiteral:
		return min(len(r.Rune)+1, limit+1)
	case syntax.OpRepeat:
		count := r.Max
		if count < 0 {
			count = r.Min + 1
		}
		child := regexCost(r.Sub[0], limit)
		if count > 0 && child > limit/count {
			return limit + 1
		}
		return min(count*child+2, limit+1)
	case syntax.OpConcat, syntax.OpAlternate:
		total := len(r.Sub) + 1
		for _, child := range r.Sub {
			total += regexCost(child, limit)
			if total > limit {
				return limit + 1
			}
		}
		return total
	default:
		total := 2
		for _, child := range r.Sub {
			total += regexCost(child, limit)
			if total > limit {
				return limit + 1
			}
		}
		return total
	}
}
