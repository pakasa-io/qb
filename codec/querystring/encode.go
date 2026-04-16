package querystring

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/pakasa-io/qb"
	"github.com/pakasa-io/qb/codec/model"
)

type pair struct {
	key   string
	value string
}

// Encode lowers a query into a canonical percent-encoded query string.
func Encode(query qb.Query, opts ...model.Option) (string, error) {
	document, err := model.BuildDocument(query, model.TransportQueryString, opts...)
	if err != nil {
		return "", err
	}

	pairs := make([]pair, 0, 16)
	appendPairs(&pairs, "", document)

	parts := make([]string, len(pairs))
	for i, item := range pairs {
		parts[i] = url.QueryEscape(item.key) + "=" + url.QueryEscape(item.value)
	}
	return strings.Join(parts, "&"), nil
}

// EncodeValues lowers a query into url.Values. Ordering is not preserved.
func EncodeValues(query qb.Query, opts ...model.Option) (url.Values, error) {
	document, err := model.BuildDocument(query, model.TransportQueryString, opts...)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	pairs := make([]pair, 0, 16)
	appendPairs(&pairs, "", document)
	for _, item := range pairs {
		values.Add(item.key, item.value)
	}
	return values, nil
}

func appendPairs(out *[]pair, prefix string, value any) {
	switch typed := value.(type) {
	case model.OrderedObject:
		for _, member := range typed {
			key := member.Key
			if prefix != "" {
				key = prefix + "[" + member.Key + "]"
			}
			appendPairs(out, key, member.Value)
		}
	case []any:
		for i, item := range typed {
			appendPairs(out, prefix+"["+strconv.Itoa(i)+"]", item)
		}
	default:
		*out = append(*out, pair{
			key:   prefix,
			value: fmt.Sprint(typed),
		})
	}
}
