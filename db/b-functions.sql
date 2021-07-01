CREATE FUNCTION earn(INOUT earned cheevo[], cheevo cheevo, golfer_id int) AS $$
BEGIN
    INSERT INTO trophies VALUES (DEFAULT, golfer_id, cheevo)
             ON CONFLICT DO NOTHING;

    IF found THEN
        earned := array_append(earned, cheevo);
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TYPE hole_rank_ret AS (strokes int, rank int, joint bool);

CREATE FUNCTION hole_rank(hole hole, lang lang, scoring scoring, golfer_id int)
RETURNS SETOF hole_rank_ret AS $$
BEGIN
    RETURN QUERY EXECUTE FORMAT(
        'WITH ranks AS (
            SELECT %I, RANK() OVER (ORDER BY %I), golfer_id
              FROM solutions
             WHERE NOT failing AND hole = $1 AND lang = $2 AND scoring = $3
        ) SELECT %I, rank::int,
                 (SELECT COUNT(*) != 1 FROM ranks r WHERE r.rank = ranks.rank)
            FROM ranks WHERE golfer_id = $4',
        scoring, scoring, scoring
    ) USING hole, lang, scoring, golfer_id;
END;
$$ LANGUAGE plpgsql;

CREATE TYPE save_solution_ret AS (
    beat_bytes      int,
    beat_chars      int,
    earned          cheevo[],
    new_bytes       int,
    new_bytes_joint bool,
    new_bytes_rank  int,
    new_chars       int,
    new_chars_joint bool,
    new_chars_rank  int,
    old_bytes       int,
    old_bytes_joint bool,
    old_bytes_rank  int,
    old_chars       int,
    old_chars_joint bool,
    old_chars_rank  int
);

CREATE FUNCTION save_solution(
    bytes int, chars int, code text, hole hole, lang lang, golfer_id int
) RETURNS save_solution_ret AS $$
#variable_conflict use_variable
DECLARE
    earned cheevo[] := '{}'::cheevo[];
    holes  int;
    rank   hole_rank_ret;
    ret    save_solution_ret;
BEGIN
    -- Ensure we're the only one messing with solutions.
    LOCK TABLE solutions IN EXCLUSIVE MODE;

    rank                := hole_rank(hole, lang, 'bytes', golfer_id);
    ret.old_bytes       := rank.strokes;
    ret.old_bytes_joint := rank.joint;
    ret.old_bytes_rank  := rank.rank;

    IF chars IS NOT NULL THEN
        rank                := hole_rank(hole, lang, 'chars', golfer_id);
        ret.old_chars       := rank.strokes;
        ret.old_chars_joint := rank.joint;
        ret.old_chars_rank  := rank.rank;
    END IF;

    -- Update the code if it's the same length or less, but only update the
    -- submitted time if the solution is shorter. This avoids a user moving
    -- down the leaderboard by matching their personal best.
    INSERT INTO solutions (bytes, chars, code, hole, lang, scoring, golfer_id)
         VALUES           (bytes, chars, code, hole, lang, 'bytes', golfer_id)
    ON CONFLICT ON CONSTRAINT solutions_pkey
    DO UPDATE SET failing = false,
                    bytes = CASE
                    WHEN solutions.failing OR excluded.bytes <= solutions.bytes
                    THEN excluded.bytes ELSE solutions.bytes END,
                    chars = CASE
                    WHEN solutions.failing OR excluded.bytes <= solutions.bytes
                    THEN excluded.chars ELSE solutions.chars END,
                     code = CASE
                    WHEN solutions.failing OR excluded.bytes <= solutions.bytes
                    THEN excluded.code ELSE solutions.code END,
                submitted = CASE
                    WHEN solutions.failing OR excluded.bytes < solutions.bytes
                    THEN excluded.submitted ELSE solutions.submitted END;

    IF chars IS NOT NULL THEN
        INSERT INTO solutions (bytes, chars, code, hole, lang, scoring, golfer_id)
             VALUES           (bytes, chars, code, hole, lang, 'chars', golfer_id)
        ON CONFLICT ON CONSTRAINT solutions_pkey
        DO UPDATE SET failing = false,
                        bytes = CASE
                        WHEN solutions.failing OR excluded.chars <= solutions.chars
                        THEN excluded.bytes ELSE solutions.bytes END,
                        chars = CASE
                        WHEN solutions.failing OR excluded.chars <= solutions.chars
                        THEN excluded.chars ELSE solutions.chars END,
                         code = CASE
                        WHEN solutions.failing OR excluded.chars <= solutions.chars
                        THEN excluded.code ELSE solutions.code END,
                    submitted = CASE
                        WHEN solutions.failing OR excluded.chars < solutions.chars
                        THEN excluded.submitted ELSE solutions.submitted END;
    END IF;

    rank                := hole_rank(hole, lang, 'bytes', golfer_id);
    ret.new_bytes       := rank.strokes;
    ret.new_bytes_joint := rank.joint;
    ret.new_bytes_rank  := rank.rank;

    IF chars IS NOT NULL THEN
        rank                := hole_rank(hole, lang, 'chars', golfer_id);
        ret.new_chars       := rank.strokes;
        ret.new_chars_joint := rank.joint;
        ret.new_chars_rank  := rank.rank;
    END IF;

    IF ret.new_bytes_rank = ret.old_bytes_rank THEN
        ret.beat_bytes = ret.old_bytes;
    ELSE
        SELECT MIN(solutions.bytes) INTO ret.beat_bytes
          FROM solutions
         WHERE solutions.hole  = hole
           AND solutions.lang  = lang
           AND solutions.bytes > bytes;
    END IF;

    IF chars IS NOT NULL THEN
        IF ret.new_chars_rank = ret.old_chars_rank THEN
             ret.beat_chars = ret.old_chars;
        ELSE
            SELECT MIN(solutions.chars) INTO ret.beat_chars
              FROM solutions
             WHERE solutions.hole  = hole
               AND solutions.lang  = lang
               AND solutions.chars > chars;
        END IF;
    END IF;

    -- Earn cheevos.
    SELECT COUNT(DISTINCT solutions.hole) INTO holes
      FROM solutions WHERE NOT failing AND solutions.golfer_id = golfer_id;

    IF holes >= 1  THEN earned := earn(earned, 'hello-world',       golfer_id); END IF;
    IF holes >= 11 THEN earned := earn(earned, 'up-to-eleven',      golfer_id); END IF;
    IF holes >= 13 THEN earned := earn(earned, 'bakers-dozen',      golfer_id); END IF;
    IF holes >= 19 THEN earned := earn(earned, 'the-watering-hole', golfer_id); END IF;
    IF holes >= 40 THEN earned := earn(earned, 'forty-winks',       golfer_id); END IF;
    IF holes >= 42 THEN earned := earn(earned, 'dont-panic',        golfer_id); END IF;
    if holes >= 50 THEN earned := earn(earned, 'bullseye',          golfer_id); END IF;

    IF hole = 'brainfuck' AND lang = 'brainfuck' THEN
        earned := earn(earned, 'inception', golfer_id);
    END IF;

    IF hole = 'fizz-buzz' THEN
        earned := earn(earned, 'interview-ready', golfer_id);
    END IF;

    IF hole = 'quine' THEN
        earned := earn(earned, 'solve-quine', golfer_id);

        IF lang = 'python' THEN
            earned := earn(earned, 'ouroboros', golfer_id);
        END IF;
    END IF;

    IF hole = 'poker' AND lang = 'fish' THEN
        earned := earn(earned, 'fish-n-chips', golfer_id);
    END IF;

    IF hole = 'ten-pin-bowling' AND lang = 'cobol' THEN
        earned := earn(earned, 'cobowl', golfer_id);
    END IF;

    IF lang = 'php' THEN
        earned := earn(earned, 'elephpant-in-the-room', golfer_id);
    END IF;

    IF hole = 'seven-segment' AND lang = 'assembly' THEN
        earned := earn(earned, 'assembly-required', golfer_id);
    END IF;

    IF (SELECT COUNT(DISTINCT solutions.code) > 1 FROM solutions
        WHERE   solutions.golfer_id = golfer_id
        AND     solutions.hole = hole
        AND     solutions.lang = lang) THEN
        earned := earn(earned, 'different-strokes', golfer_id);
    END IF;

    ret.earned := earned;

    RETURN ret;
END;
$$ LANGUAGE plpgsql;
