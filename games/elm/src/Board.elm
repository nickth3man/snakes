module Board exposing
    ( Board
    , Dir(..)
    , Pos
    , delta
    , dirs
    , inBounds
    , key
    , opposite
    , step
    , wallAdjacent
    )

{-| The vocabulary the rules and the demo AI share: positions, directions and
a snapshot of the board they both reason about.
-}

import Set exposing (Set)


type alias Pos =
    { x : Int, y : Int }


type Dir
    = Up
    | Right
    | Down
    | Left


{-| UP, RIGHT, DOWN, LEFT — the order decides ties in the AI, so it matters.
-}
dirs : List Dir
dirs =
    [ Up, Right, Down, Left ]


delta : Dir -> Pos
delta dir =
    case dir of
        Up ->
            { x = 0, y = -1 }

        Right ->
            { x = 1, y = 0 }

        Down ->
            { x = 0, y = 1 }

        Left ->
            { x = -1, y = 0 }


opposite : Dir -> Dir -> Bool
opposite a b =
    delta a == { x = -(delta b).x, y = -(delta b).y }


step : Dir -> Pos -> Pos
step dir pos =
    let
        d =
            delta dir
    in
    { x = pos.x + d.x, y = pos.y + d.y }


key : Pos -> ( Int, Int )
key pos =
    ( pos.x, pos.y )


{-| Everything a move depends on, and nothing else.
-}
type alias Board =
    { cols : Int
    , rows : Int
    , snake : List Pos
    , occupied : Set ( Int, Int )
    , food : Maybe Pos
    , dir : Dir
    }


inBounds : Board -> Pos -> Bool
inBounds board p =
    p.x >= 0 && p.y >= 0 && p.x < board.cols && p.y < board.rows


wallAdjacent : Board -> Pos -> Bool
wallAdjacent board p =
    p.x == 0 || p.y == 0 || p.x == board.cols - 1 || p.y == board.rows - 1
