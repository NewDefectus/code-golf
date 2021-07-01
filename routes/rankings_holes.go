package routes

import (
	"net/http"
	"time"

	"github.com/code-golf/code-golf/hole"
	"github.com/code-golf/code-golf/lang"
	"github.com/code-golf/code-golf/pager"
	"github.com/code-golf/code-golf/session"
)

// RankingsHoles serves GET /rankings/holes/{hole}/{lang}/{scoring}
func RankingsHoles(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Country, Lang, Login         string
		Holes, Rank, Points, Strokes int
		Submitted                    time.Time
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
		data.Scoring != "chars" && data.Scoring != "bytes" {
		NotFound(w, r)
		return
	}

	var distinct, table string

	if data.HoleID == "all" {
		distinct = "DISTINCT ON (hole, golfer_id)"
		table = "summed_leaderboard"
	} else {
		table = "scored_leaderboard"
	}

	rows, err := session.Database(r).Query(
		`WITH leaderboard AS (
		  SELECT `+distinct+`
		         hole,
		         submitted,
		         `+data.Scoring+` strokes,
		         golfer_id,
		         lang
		    FROM solutions
		   WHERE NOT failing
		     AND $1 IN ('all', hole::text)
		     AND $2 IN ('all', lang::text)
		     AND scoring = $3
		ORDER BY hole, golfer_id, `+data.Scoring+`, submitted
		), scored_leaderboard AS (
		  SELECT l.hole,
		         1 holes,
		         lang,
		         ROUND(
		             (COUNT(*) OVER (PARTITION BY l.hole) -
		                RANK() OVER (PARTITION BY l.hole ORDER BY strokes) + 1)
		             * (1000.0 / COUNT(*) OVER (PARTITION BY l.hole))
		         ) points,
		         strokes,
		         submitted,
		         l.golfer_id
		    FROM leaderboard l
		), summed_leaderboard AS (
		  SELECT golfer_id,
		         COUNT(*)       holes,
		         ''             lang,
		         SUM(points)    points,
		         SUM(strokes)   strokes,
		         MAX(submitted) submitted
		    FROM scored_leaderboard
		GROUP BY golfer_id
		) SELECT COALESCE(CASE WHEN show_country THEN country END, ''),
		         holes,
		         lang,
		         login,
		         points,
		         RANK() OVER (ORDER BY points DESC, strokes),
		         strokes,
		         submitted,
		         COUNT(*) OVER()
		    FROM `+table+`
		    JOIN golfers on golfer_id = id
		ORDER BY points DESC, strokes, submitted
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
			&r.Country,
			&r.Holes,
			&r.Lang,
			&r.Login,
			&r.Points,
			&r.Rank,
			&r.Strokes,
			&r.Submitted,
			&data.Pager.Total,
		); err != nil {
			panic(err)
		}

		r.Lang = lang.ByID[r.Lang].Name

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
		description += lang.Name + " in "
	} else {
		description += "all languages in "
	}

	description += data.Scoring

	render(w, r, "rankings/holes", data, "Rankings: Holes", description)
}
