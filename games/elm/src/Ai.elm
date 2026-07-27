module Ai exposing (Decision, Tier(..), decide)

{-| The demo-mode AI, a direct port of the TypeScript demo controller.

Tier 1: safe food pursuit — a move that can reach the food by BFS while leaving
at least 1.2x the snake's length of reachable cells. Ties go to the shorter
path, then more wall-hugging, then more turns, all for the show.

Tier 2: maximise open space — the move with the largest flood fill. Ties go to
wall-adjacent cells, then to whatever lands nearer the food.

Fallback: the legal move that lands nearest the food.

-}

import Board exposing (Board, Dir, Pos, dirs, inBounds, key, opposite, step, wallAdjacent)
import Dict exposing (Dict)
import Set exposing (Set)


type Tier
    = Tier1
    | Tier2
    | Fallback


{-| The chosen move plus everything the AI-vision overlay draws.
-}
type alias Decision =
    { dir : Dir
    , tier : Tier
    , path : List Pos
    , reachable : List Pos
    }


decide : Board -> Decision
decide board =
    let
        head =
            List.head board.snake |> Maybe.withDefault { x = 0, y = 0 }

        moves =
            dirs
                |> List.filter
                    (\d -> not (List.length board.snake > 1 && opposite d board.dir))
                |> List.map (\d -> ( d, step d head ))
                |> List.filter
                    (\( _, p ) -> inBounds board p && not (Set.member (key p) board.occupied))

        -- After one step the tail moves on, so it is not an obstacle here.
        blocked =
            board.snake
                |> List.take (List.length board.snake - 1)
                |> List.map key
                |> Set.fromList
    in
    case moves of
        [] ->
            { dir = board.dir, tier = Fallback, path = [], reachable = [] }

        [ ( only, next ) ] ->
            { dir = only
            , tier = Fallback
            , path = []
            , reachable = flood board blocked next
            }

        _ ->
            decideMulti board head moves blocked


decideMulti : Board -> Pos -> List ( Dir, Pos ) -> Set ( Int, Int ) -> Decision
decideMulti board head moves blocked =
    let
        tier1 =
            case board.food of
                Nothing ->
                    Nothing

                Just food ->
                    moves
                        |> List.filterMap
                            (\( d, next ) ->
                                case bfs board blocked next food of
                                    Nothing ->
                                        Nothing

                                    Just path ->
                                        let
                                            space =
                                                List.length (flood board blocked next)
                                        in
                                        -- space < length * 1.2, in integers.
                                        if space * 5 < List.length board.snake * 6 then
                                            Nothing

                                        else
                                            Just
                                                ( ( List.length path
                                                  , negate (wallHugs board path)
                                                  , negate (countTurns path)
                                                  )
                                                , ( d, next, path )
                                                )
                            )
                        |> bestBy
    in
    case tier1 of
        Just ( d, next, path ) ->
            { dir = d
            , tier = Tier1
            , path = head :: path
            , reachable = flood board blocked next
            }

        Nothing ->
            let
                scored =
                    moves
                        |> List.map
                            (\( d, next ) ->
                                let
                                    space =
                                        List.length (flood board blocked next)

                                    hug =
                                        if wallAdjacent board next then
                                            1

                                        else
                                            0

                                    dist =
                                        case board.food of
                                            Just f ->
                                                abs (f.x - next.x) + abs (f.y - next.y)

                                            Nothing ->
                                                0
                                in
                                ( ( negate space, negate hug, dist ), ( d, next, space ) )
                            )
                        |> bestBy
            in
            case scored of
                Just ( d, next, space ) ->
                    if space > 1 then
                        { dir = d
                        , tier = Tier2
                        , path = []
                        , reachable = flood board blocked next
                        }

                    else
                        fallback board moves blocked

                Nothing ->
                    fallback board moves blocked


fallback : Board -> List ( Dir, Pos ) -> Set ( Int, Int ) -> Decision
fallback board moves blocked =
    let
        ranked =
            case board.food of
                Just food ->
                    moves
                        |> List.map
                            (\( d, next ) ->
                                ( ( abs (food.x - next.x) + abs (food.y - next.y), 0, 0 )
                                , ( d, next, 0 )
                                )
                            )
                        |> bestBy

                Nothing ->
                    List.head moves |> Maybe.map (\( d, next ) -> ( d, next, 0 ))
    in
    case ranked of
        Just ( d, next, _ ) ->
            { dir = d, tier = Fallback, path = [], reachable = flood board blocked next }

        Nothing ->
            { dir = board.dir, tier = Fallback, path = [], reachable = [] }


{-| First entry with the smallest key — ties keep the earlier direction, which
is what the TypeScript sort does.
-}
bestBy : List ( ( Int, Int, Int ), a ) -> Maybe a
bestBy entries =
    List.foldl
        (\( k, v ) acc ->
            case acc of
                Nothing ->
                    Just ( k, v )

                Just ( bestKey, _ ) ->
                    if k < bestKey then
                        Just ( k, v )

                    else
                        acc
        )
        Nothing
        entries
        |> Maybe.map Tuple.second


wallHugs : Board -> List Pos -> Int
wallHugs board path =
    List.length (List.filter (wallAdjacent board) path)


countTurns : List Pos -> Int
countTurns path =
    case path of
        a :: b :: rest ->
            let
                walk prev from remaining acc =
                    case remaining of
                        [] ->
                            acc

                        next :: tail ->
                            let
                                d =
                                    { x = next.x - from.x, y = next.y - from.y }
                            in
                            if d /= prev then
                                walk d next tail (acc + 1)

                            else
                                walk prev next tail acc
            in
            walk { x = b.x - a.x, y = b.y - a.y } b rest 0

        _ ->
            0


{-| Shortest path from start to goal inclusive, or Nothing.
-}
bfs : Board -> Set ( Int, Int ) -> Pos -> Pos -> Maybe (List Pos)
bfs board blocked start goal =
    if Set.member (key start) blocked then
        Nothing

    else if start == goal then
        Just [ start ]

    else
        bfsLoop board blocked start goal [ start ] (Dict.singleton (key start) (key start))


bfsLoop :
    Board
    -> Set ( Int, Int )
    -> Pos
    -> Pos
    -> List Pos
    -> Dict ( Int, Int ) ( Int, Int )
    -> Maybe (List Pos)
bfsLoop board blocked start goal frontier parents =
    case frontier of
        [] ->
            Nothing

        _ ->
            let
                expand pos ( acc, seen, found ) =
                    if found /= Nothing then
                        ( acc, seen, found )

                    else
                        List.foldl
                            (\d ( acc2, seen2, found2 ) ->
                                let
                                    next =
                                        step d pos
                                in
                                if found2 /= Nothing then
                                    ( acc2, seen2, found2 )

                                else if
                                    not (inBounds board next)
                                        || Set.member (key next) blocked
                                        || Dict.member (key next) seen2
                                then
                                    ( acc2, seen2, found2 )

                                else
                                    let
                                        seen3 =
                                            Dict.insert (key next) (key pos) seen2
                                    in
                                    if next == goal then
                                        ( acc2, seen3, Just (walkBack start next seen3) )

                                    else
                                        ( next :: acc2, seen3, found2 )
                            )
                            ( acc, seen, found )
                            dirs

                ( nextFrontier, nextParents, result ) =
                    List.foldl expand ( [], parents, Nothing ) frontier
            in
            case result of
                Just path ->
                    Just path

                Nothing ->
                    bfsLoop board blocked start goal (List.reverse nextFrontier) nextParents


walkBack : Pos -> Pos -> Dict ( Int, Int ) ( Int, Int ) -> List Pos
walkBack start goal parents =
    let
        climb ( x, y ) acc =
            let
                pos =
                    { x = x, y = y }
            in
            if pos == start then
                pos :: acc

            else
                case Dict.get ( x, y ) parents of
                    Just parent ->
                        climb parent (pos :: acc)

                    Nothing ->
                        pos :: acc
    in
    climb (key goal) []


{-| Every cell reachable from start without crossing an obstacle.
-}
flood : Board -> Set ( Int, Int ) -> Pos -> List Pos
flood board blocked start =
    if Set.member (key start) blocked then
        []

    else
        floodLoop board blocked [ start ] (Set.singleton (key start)) [ start ]


floodLoop : Board -> Set ( Int, Int ) -> List Pos -> Set ( Int, Int ) -> List Pos -> List Pos
floodLoop board blocked frontier seen acc =
    case frontier of
        [] ->
            acc

        _ ->
            let
                expand pos ( next, seen2 ) =
                    List.foldl
                        (\d ( next2, seen3 ) ->
                            let
                                candidate =
                                    step d pos
                            in
                            if
                                not (inBounds board candidate)
                                    || Set.member (key candidate) blocked
                                    || Set.member (key candidate) seen3
                            then
                                ( next2, seen3 )

                            else
                                ( candidate :: next2, Set.insert (key candidate) seen3 )
                        )
                        ( next, seen2 )
                        dirs

                ( newFrontier, newSeen ) =
                    List.foldl expand ( [], seen ) frontier
            in
            floodLoop board blocked newFrontier newSeen (acc ++ newFrontier)
