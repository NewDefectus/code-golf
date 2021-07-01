package routes

import (
	"net/http"

	"github.com/code-golf/code-golf/hole"
	"github.com/code-golf/code-golf/lang"
	"github.com/code-golf/code-golf/pager"
	"github.com/code-golf/code-golf/session"
)

// RankingsMedals serves GET /rankings/medals/{hole}/{lang}
func RankingsMedals(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Country, Login                      string
		Rank, Diamond, Gold, Silver, Bronze int
	}

	data := struct {
		HoleID, LangID, Scoring string
		Holes                   []hole.Hole
		Langs                   []lang.Lang
		Pager                   *pager.Pager
		Rows                    []row
	}{
		HoleID:  param(r, "hole"),
		Holes:   hole.List,
		LangID:  param(r, "lang"),
		Langs:   lang.List,
		Pager:   pager.New(r),
		Rows:    make([]row, 0, pager.PerPage),
		Scoring: param(r, "scoring"),
	}

	if data.HoleID != "all" && hole.ByID[data.HoleID].ID == "" ||
		data.LangID != "all" && lang.ByID[data.LangID].ID == "" ||
		data.Scoring != "all" && data.Scoring != "chars" && data.Scoring != "bytes" {
		NotFound(w, r)
		return
	}

	rows, err := session.Database(r).Query(
		`WITH counts AS (
		    SELECT golfer_id,
		           COUNT(*) FILTER (WHERE medal = 'diamond') diamond,
		           COUNT(*) FILTER (WHERE medal = 'gold'   ) gold,
		           COUNT(*) FILTER (WHERE medal = 'silver' ) silver,
		           COUNT(*) FILTER (WHERE medal = 'bronze' ) bronze
		      FROM medals
		     WHERE $1 IN ('all', hole::text)
		       AND $2 IN ('all', lang::text)
		       AND $3 IN ('all', scoring::text)
		  GROUP BY golfer_id
		) SELECT RANK() OVER(
		             ORDER BY gold DESC, diamond DESC, silver DESC, bronze DESC
		         ),
		         COALESCE(CASE WHEN show_country THEN country END, ''),
		         login,
		         diamond,
		         gold,
		         silver,
		         bronze,
		         COUNT(*) OVER()
		    FROM counts
		    JOIN golfers ON id = golfer_id
		ORDER BY rank, login
		   LIMIT $4 OFFSET $5`,
		data.HoleID,
		data.LangID,
		data.Scoring,
		pager.PerPage,
		data.Pager.Offset,
	)
	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		var r row

		if err := rows.Scan(
			&r.Rank,
			&r.Country,
			&r.Login,
			&r.Diamond,
			&r.Gold,
			&r.Silver,
			&r.Bronze,
			&data.Pager.Total,
		); err != nil {
			panic(err)
		}

		data.Rows = append(data.Rows, r)
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}

	if data.Pager.Calculate() {
		NotFound(w, r)
		return
	}

	description := ""
	if hole := hole.ByID[data.HoleID]; hole.ID != "" {
		description += hole.Name + " in "
	} else {
		description += "All holes in "
	}

	if lang := lang.ByID[data.LangID]; lang.ID != "" {
		description += lang.Name
	} else {
		description += "all languages"
	}

	if data.Scoring != "all" {
		description += " in " + data.Scoring
	}

	render(w, r, "rankings/medals", data, "Rankings: Medals", description)
}
