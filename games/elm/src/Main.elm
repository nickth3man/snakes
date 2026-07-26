port module Main exposing (main)

{-| Snake in Elm.

The odd one out in this repo: there is no canvas and no imperative draw loop.
The board is a value, `update` returns the next board, and the view is a pure
function from that value to SVG. The only side effects are the tick
subscription, key events, and a port that stashes the high score.

-}

import Browser
import Browser.Events
import Html exposing (Html, a, b, div, footer, h1, h2, header, kbd, p, span, text)
import Html.Attributes exposing (class, hidden, href)
import Html.Events
import Json.Decode as Decode exposing (Decoder)
import Random
import Set exposing (Set)
import Svg exposing (Svg)
import Svg.Attributes as SA
import Time



-- BOARD


cols : Int
cols =
    24


rows : Int
rows =
    24


cellPx : Int
cellPx =
    24


type alias Pos =
    { x : Int, y : Int }


type Dir
    = Up
    | Right
    | Down
    | Left


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
opposite a b_ =
    delta a == { x = -(delta b_).x, y = -(delta b_).y }



-- MODEL


type State
    = Ready
    | Playing
    | Paused
    | Finished Bool


type alias Model =
    { snake : List Pos
    , food : Pos
    , dir : Dir
    , pending : Dir
    , score : Int
    , best : Int
    , state : State
    , seed : Random.Seed
    , touch : Maybe ( Float, Float )
    }


type alias Flags =
    { best : Int, seed : Int }


startSnake : List Pos
startSnake =
    [ { x = 4, y = rows // 2 }, { x = 3, y = rows // 2 }, { x = 2, y = rows // 2 } ]


init : Flags -> ( Model, Cmd Msg )
init flags =
    let
        ( food, seed ) =
            spawnFood startSnake (Random.initialSeed flags.seed)
    in
    ( { snake = startSnake
      , food = food
      , dir = Right
      , pending = Right
      , score = 0
      , best = flags.best
      , state = Ready
      , seed = seed
      , touch = Nothing
      }
    , Cmd.none
    )


restart : Model -> Model
restart model =
    let
        ( food, seed ) =
            spawnFood startSnake model.seed
    in
    { model
        | snake = startSnake
        , food = food
        , dir = Right
        , pending = Right
        , score = 0
        , state = Playing
        , seed = seed
    }


{-| Pick uniformly among the cells the snake does not occupy.
-}
spawnFood : List Pos -> Random.Seed -> ( Pos, Random.Seed )
spawnFood snake seed =
    let
        taken : Set ( Int, Int )
        taken =
            Set.fromList (List.map (\p -> ( p.x, p.y )) snake)

        free : List Pos
        free =
            List.filter (\p -> not (Set.member ( p.x, p.y ) taken)) allCells
    in
    case free of
        [] ->
            ( { x = 0, y = 0 }, seed )

        first :: _ ->
            let
                ( i, next ) =
                    Random.step (Random.int 0 (List.length free - 1)) seed
            in
            ( Maybe.withDefault first (List.head (List.drop i free)), next )


allCells : List Pos
allCells =
    List.range 0 (rows - 1)
        |> List.concatMap (\y -> List.map (\x -> { x = x, y = y }) (List.range 0 (cols - 1)))



-- UPDATE


type Msg
    = Tick Time.Posix
    | Key String
    | TouchStart Float Float
    | TouchEnd Float Float
    | Click


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Tick _ ->
            let
                next =
                    step model
            in
            ( next, saveIfBest model next )

        Key code ->
            ( handleKey code model, Cmd.none )

        TouchStart x y ->
            ( { model | touch = Just ( x, y ) }, Cmd.none )

        TouchEnd x y ->
            ( handleSwipe x y model, Cmd.none )

        Click ->
            ( startIfIdle model, Cmd.none )


saveIfBest : Model -> Model -> Cmd Msg
saveIfBest before after =
    case ( before.state, after.state ) of
        ( Playing, Finished _ ) ->
            if after.score > after.best then
                saveBest after.score

            else
                Cmd.none

        _ ->
            Cmd.none


startIfIdle : Model -> Model
startIfIdle model =
    case model.state of
        Playing ->
            model

        Paused ->
            model

        _ ->
            restart model


turn : Dir -> Model -> Model
turn dir model =
    case model.state of
        Playing ->
            if List.length model.snake > 1 && opposite dir model.dir then
                model

            else
                { model | pending = dir }

        _ ->
            restart model


handleKey : String -> Model -> Model
handleKey code model =
    case code of
        "ArrowUp" ->
            turn Up model

        "KeyW" ->
            turn Up model

        "ArrowRight" ->
            turn Right model

        "KeyD" ->
            turn Right model

        "ArrowDown" ->
            turn Down model

        "KeyS" ->
            turn Down model

        "ArrowLeft" ->
            turn Left model

        "KeyA" ->
            turn Left model

        "Space" ->
            togglePause model

        "Enter" ->
            togglePause model

        "KeyP" ->
            togglePause model

        _ ->
            model


togglePause : Model -> Model
togglePause model =
    case model.state of
        Playing ->
            { model | state = Paused }

        Paused ->
            { model | state = Playing }

        _ ->
            restart model


handleSwipe : Float -> Float -> Model -> Model
handleSwipe x y model =
    case model.touch of
        Nothing ->
            model

        Just ( sx, sy ) ->
            let
                ( dx, dy ) =
                    ( x - sx, y - sy )

                cleared =
                    { model | touch = Nothing }
            in
            if abs dx < 24 && abs dy < 24 then
                startIfIdle cleared

            else if abs dx > abs dy then
                turn
                    (if dx > 0 then
                        Right

                     else
                        Left
                    )
                    cleared

            else
                turn
                    (if dy > 0 then
                        Down

                     else
                        Up
                    )
                    cleared


{-| One move. Walls and the snake's own body are fatal; the very last segment
is not, because it steps out of the way on the same tick.
-}
step : Model -> Model
step model =
    let
        dir =
            if List.length model.snake > 1 && opposite model.pending model.dir then
                model.dir

            else
                model.pending

        d =
            delta dir

        next =
            case List.head model.snake of
                Just h ->
                    { x = h.x + d.x, y = h.y + d.y }

                Nothing ->
                    { x = 0, y = 0 }

        ate =
            next == model.food

        kept =
            if ate then
                model.snake

            else
                List.take (List.length model.snake - 1) model.snake

        offBoard =
            next.x < 0 || next.y < 0 || next.x >= cols || next.y >= rows
    in
    if offBoard || List.member next kept then
        { model | dir = dir, state = Finished False }

    else if ate then
        let
            grown =
                next :: kept

            ( food, seed ) =
                spawnFood grown model.seed
        in
        { model
            | snake = grown
            , food = food
            , seed = seed
            , dir = dir
            , score = model.score + 10
            , best = max model.best (model.score + 10)
            , state =
                if List.length grown == cols * rows then
                    Finished True

                else
                    Playing
        }

    else
        { model | snake = next :: kept, dir = dir }


{-| Milliseconds per move: quicker with every apple, down to a floor.
-}
tickMs : Model -> Float
tickMs model =
    max 65 (120 - 2 * toFloat (model.score // 10))



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ case model.state of
            Playing ->
                Time.every (tickMs model) Tick

            _ ->
                Sub.none
        , Browser.Events.onKeyDown (Decode.map Key (Decode.field "code" Decode.string))
        ]



-- VIEW


view : Model -> Html Msg
view model =
    div [ class "shell" ]
        [ header []
            [ h1 [] [ text "SNAKE ", span [ class "lang" ] [ text "// ELM" ] ]
            , div [ class "hud" ]
                [ span [] [ text "SCORE ", b [] [ text (String.fromInt model.score) ] ]
                , span [] [ text "BEST ", b [] [ text (String.fromInt model.best) ] ]
                ]
            ]
        , div
            [ class "stage"
            , Html.Events.on "touchstart" (touchDecoder "touches" TouchStart)
            , Html.Events.on "touchend" (touchDecoder "changedTouches" TouchEnd)
            , Html.Events.onClick Click
            ]
            [ board model, overlay model ]
        , footer []
            [ span []
                [ kbd [] [ text "←↑↓→" ]
                , text " / "
                , kbd [] [ text "WASD" ]
                , text " move · "
                , kbd [] [ text "P" ]
                , text " pause · swipe on mobile"
                ]
            , a [ href "../" ] [ text "← all snakes" ]
            ]
        ]


touchDecoder : String -> (Float -> Float -> Msg) -> Decoder Msg
touchDecoder list toMsg =
    Decode.map2 toMsg
        (Decode.at [ list, "0", "clientX" ] Decode.float)
        (Decode.at [ list, "0", "clientY" ] Decode.float)


board : Model -> Html Msg
board model =
    let
        size =
            String.fromInt (cols * cellPx)
    in
    Svg.svg
        [ SA.viewBox ("0 0 " ++ size ++ " " ++ String.fromInt (rows * cellPx))
        , SA.width "100%"
        , SA.height "100%"
        ]
        (Svg.rect
            [ SA.width "100%", SA.height "100%", SA.fill "#1a1a2e" ]
            []
            :: gridLines
            ++ (apple model.food :: snakeParts model.snake)
        )


gridLines : List (Svg Msg)
gridLines =
    let
        w =
            cols * cellPx

        h =
            rows * cellPx

        vertical i =
            Svg.line
                [ SA.x1 (String.fromInt (i * cellPx))
                , SA.y1 "0"
                , SA.x2 (String.fromInt (i * cellPx))
                , SA.y2 (String.fromInt h)
                , SA.stroke "#16213e"
                ]
                []

        horizontal j =
            Svg.line
                [ SA.x1 "0"
                , SA.y1 (String.fromInt (j * cellPx))
                , SA.x2 (String.fromInt w)
                , SA.y2 (String.fromInt (j * cellPx))
                , SA.stroke "#16213e"
                ]
                []
    in
    List.map vertical (List.range 1 (cols - 1))
        ++ List.map horizontal (List.range 1 (rows - 1))


apple : Pos -> Svg Msg
apple pos =
    Svg.circle
        [ SA.cx (String.fromFloat (toFloat (pos.x * cellPx) + toFloat cellPx / 2))
        , SA.cy (String.fromFloat (toFloat (pos.y * cellPx) + toFloat cellPx / 2))
        , SA.r (String.fromFloat (toFloat cellPx * 0.32))
        , SA.fill "#ff7675"
        ]
        []


snakeParts : List Pos -> List (Svg Msg)
snakeParts snake =
    case snake of
        [] ->
            []

        head :: body ->
            List.map (segment "#6c5ce7" 2 5) body ++ headPart head


segment : String -> Int -> Int -> Pos -> Svg Msg
segment color inset radius pos =
    Svg.rect
        [ SA.x (String.fromInt (pos.x * cellPx + inset))
        , SA.y (String.fromInt (pos.y * cellPx + inset))
        , SA.width (String.fromInt (cellPx - 2 * inset))
        , SA.height (String.fromInt (cellPx - 2 * inset))
        , SA.rx (String.fromInt radius)
        , SA.fill color
        ]
        []


headPart : Pos -> List (Svg Msg)
headPart pos =
    let
        eye offset =
            Svg.circle
                [ SA.cx (String.fromFloat (toFloat (pos.x * cellPx) + toFloat cellPx * offset))
                , SA.cy (String.fromFloat (toFloat (pos.y * cellPx) + toFloat cellPx * 0.38))
                , SA.r (String.fromFloat (toFloat cellPx * 0.09))
                , SA.fill "#0a0a1e"
                ]
                []
    in
    [ segment "#00cec9" 1 6 pos, eye 0.35, eye 0.65 ]


overlay : Model -> Html Msg
overlay model =
    let
        panel title body =
            div [ class "overlay" ] [ h2 [] [ text title ], p [] body ]

        finishedBody =
            [ text ("Score " ++ String.fromInt model.score ++ " · length " ++ String.fromInt (List.length model.snake))
            , Html.br [] []
            , text "Press "
            , kbd [] [ text "SPACE" ]
            , text " or tap to play again"
            ]
    in
    case model.state of
        Ready ->
            panel "ELM SNAKE"
                [ text "Press ", kbd [] [ text "SPACE" ], text " or tap to start" ]

        Paused ->
            panel "PAUSED" [ text "Press ", kbd [] [ text "P" ], text " to resume" ]

        Finished True ->
            panel "PERFECT GAME" finishedBody

        Finished False ->
            panel "GAME OVER" finishedBody

        Playing ->
            div [ class "overlay", hidden True ] []



-- PORTS


port saveBest : Int -> Cmd msg


main : Program Flags Model Msg
main =
    Browser.element
        { init = init
        , update = update
        , view = view
        , subscriptions = subscriptions
        }
