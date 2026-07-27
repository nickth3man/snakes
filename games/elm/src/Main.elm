port module Main exposing (main)

{-| Snake in Elm.

The odd one out in this repo: there is no canvas and no imperative draw loop.
The board is a value, `update` returns the next board, and the view is a pure
function from that value to SVG. Both modes are here — the menu, the rules and
the three-tier demo AI with its vision overlay — with side effects limited to
subscriptions and a single port that stashes the high score.

-}

import Ai exposing (Decision, Tier(..), decide)
import Board exposing (Board, Dir(..), Pos, inBounds, key, opposite, step)
import Browser
import Browser.Events
import Html exposing (Html, a, button, div, footer, h1, kbd, p, span, text)
import Html.Attributes exposing (class, classList, hidden, href, id, style, title)
import Html.Events
import Html.Keyed
import Json.Decode as Decode exposing (Decoder)
import Random
import Set exposing (Set)
import Svg
import Svg.Attributes as SA
import Time



-- CONSTANTS


cellPx : Int
cellPx =
    24


tickMs : Float
tickMs =
    130


{-| Roughly 2.5 seconds at one tick per 130 ms.
-}
badgeTicks : Int
badgeTicks =
    19


-- MODEL


type Screen
    = Menu
    | Playing


type Mode
    = Normal
    | Demo


type alias Particle =
    { x : Float, y : Float, vx : Float, vy : Float, life : Float }


type alias Model =
    { screen : Screen
    , mode : Mode
    , cols : Int
    , rows : Int
    , snake : List Pos
    , occupied : Set ( Int, Int )
    , food : Maybe Pos
    , dir : Dir
    , queue : List Dir
    , score : Int
    , best : Int
    , alive : Bool
    , won : Bool
    , seed : Random.Seed
    , decision : Maybe Decision
    , showVision : Bool
    , badge : Int
    , lastTier : Maybe Tier
    , particles : List Particle
    , foodPop : Float
    , headPulse : Float
    , viewport : { width : Int, height : Int }
    , touch : Maybe ( Float, Float )
    , touchFired : Bool
    , aiStats : String
    , deaths : Int
    }


type alias Flags =
    { best : Int
    , seed : Int
    , width : Int
    , height : Int
    , aiStats : String
    }


init : Flags -> ( Model, Cmd Msg )
init flags =
    ( { screen = Menu
      , mode = Normal
      , cols = 30
      , rows = 22
      , snake = []
      , occupied = Set.empty
      , food = Nothing
      , dir = Right
      , queue = []
      , score = 0
      , best = flags.best
      , alive = False
      , won = False
      , seed = Random.initialSeed flags.seed
      , decision = Nothing
      , showVision = False
      , badge = 0
      , lastTier = Nothing
      , particles = []
      , foodPop = 1
      , headPulse = 1
      , viewport = { width = flags.width, height = flags.height }
      , touch = Nothing
      , touchFired = False
      , aiStats = flags.aiStats
      , deaths = 0
      }
    , Cmd.none
    )


{-| Deal the opening position: three cells, mid-row, facing right. The board
shape follows the viewport, then stays put for the round.
-}
startGame : Mode -> Model -> Model
startGame mode model =
    let
        portrait =
            model.viewport.height > model.viewport.width

        ( cols, rows ) =
            if portrait then
                ( 18, 38 )

            else
                ( 30, 22 )

        mid =
            rows // 2

        safe =
            max (min (cols // 2) 5) 2

        snake =
            [ { x = safe, y = mid }, { x = safe - 1, y = mid }, { x = safe - 2, y = mid } ]

        occupied =
            Set.fromList (List.map key snake)

        ( food, seed ) =
            spawnFood cols rows occupied model.seed
    in
    { model
        | screen = Playing
        , mode = mode
        , cols = cols
        , rows = rows
        , snake = snake
        , occupied = occupied
        , food = food
        , dir = Right
        , queue = []
        , score = 0
        , alive = True
        , won = False
        , seed = seed
        , decision = Nothing
        , showVision = False
        , badge = 0
        , lastTier = Nothing
        , particles = []
        , foodPop = 1
        , headPulse = 1
        , touch = Nothing
        , touchFired = False
    }


{-| Pick uniformly among the cells the snake does not occupy.
-}
spawnFood : Int -> Int -> Set ( Int, Int ) -> Random.Seed -> ( Maybe Pos, Random.Seed )
spawnFood cols rows occupied seed =
    let
        free =
            List.filter (\p -> not (Set.member (key p) occupied)) (allCells cols rows)
    in
    case free of
        [] ->
            ( Nothing, seed )

        first :: _ ->
            let
                ( i, next ) =
                    Random.step (Random.int 0 (List.length free - 1)) seed
            in
            ( Just (Maybe.withDefault first (List.head (List.drop i free))), next )


allCells : Int -> Int -> List Pos
allCells cols rows =
    List.range 0 (rows - 1)
        |> List.concatMap (\y -> List.map (\x -> { x = x, y = y }) (List.range 0 (cols - 1)))


{-| The slice of the model the rules and the AI actually reason about.
-}
boardOf : Model -> Board
boardOf model =
    { cols = model.cols
    , rows = model.rows
    , snake = model.snake
    , occupied = model.occupied
    , food = model.food
    , dir = model.dir
    }



-- UPDATE


type Msg
    = Tick Time.Posix
    | Frame Float
    | KeyDown String
    | Start Mode
    | OpenMenu
    | ToggleVision
    | Restart
    | TouchStart Float Float
    | TouchMove Float Float
    | TouchEnd
    | Resize Int Int


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Tick _ ->
            let
                next =
                    advance model
            in
            ( next
            , if next.alive == False && model.alive && next.score > model.best then
                saveBest next.score

              else
                Cmd.none
            )

        Frame dt ->
            ( animate dt model, Cmd.none )

        KeyDown code ->
            handleKey code model

        Start mode ->
            ( startGame mode model, Cmd.none )

        OpenMenu ->
            ( { model | screen = Menu, alive = False }, Cmd.none )

        ToggleVision ->
            ( toggleVision model, Cmd.none )

        Restart ->
            if model.screen == Playing && not model.alive then
                ( startGame model.mode model, Cmd.none )

            else
                ( model, Cmd.none )

        TouchStart x y ->
            ( { model | touch = Just ( x, y ), touchFired = False }, Cmd.none )

        TouchMove x y ->
            ( handleSwipe x y model, Cmd.none )

        TouchEnd ->
            if model.touchFired then
                ( { model | touch = Nothing }, Cmd.none )

            else
                update Restart { model | touch = Nothing }

        Resize w h ->
            ( { model | viewport = { width = w, height = h } }, Cmd.none )


toggleVision : Model -> Model
toggleVision model =
    if model.mode /= Demo || model.screen /= Playing then
        model

    else if model.showVision then
        { model | showVision = False, badge = 0, lastTier = Nothing }

    else
        { model | showVision = True }


handleKey : String -> Model -> ( Model, Cmd Msg )
handleKey code model =
    case model.screen of
        Menu ->
            case code of
                "KeyN" ->
                    ( startGame Normal model, Cmd.none )

                "KeyD" ->
                    ( startGame Demo model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        Playing ->
            case ( code, model.mode ) of
                ( "KeyM", _ ) ->
                    ( { model | screen = Menu, alive = False }, Cmd.none )

                ( "KeyV", _ ) ->
                    ( toggleVision model, Cmd.none )

                ( "Space", _ ) ->
                    update Restart model

                ( "Enter", _ ) ->
                    update Restart model

                ( _, Demo ) ->
                    ( model, Cmd.none )

                _ ->
                    ( turn code model, Cmd.none )


turn : String -> Model -> Model
turn code model =
    let
        dir =
            case code of
                "ArrowUp" ->
                    Just Up

                "KeyW" ->
                    Just Up

                "ArrowRight" ->
                    Just Right

                "KeyD" ->
                    Just Right

                "ArrowDown" ->
                    Just Down

                "KeyS" ->
                    Just Down

                "ArrowLeft" ->
                    Just Left

                "KeyA" ->
                    Just Left

                _ ->
                    Nothing
    in
    case dir of
        Just d ->
            queueDir d model

        Nothing ->
            model


{-| Buffer a player direction, at most two deep, refusing reversals.
-}
queueDir : Dir -> Model -> Model
queueDir dir model =
    if not model.alive || List.length model.queue >= 2 then
        model

    else
        let
            last =
                List.head (List.reverse model.queue) |> Maybe.withDefault model.dir
        in
        if opposite dir last then
            model

        else
            { model | queue = model.queue ++ [ dir ] }


handleSwipe : Float -> Float -> Model -> Model
handleSwipe x y model =
    case ( model.touch, model.touchFired, model.mode ) of
        ( Just ( sx, sy ), False, Normal ) ->
            let
                ( dx, dy ) =
                    ( x - sx, y - sy )
            in
            if abs dx < 24 && abs dy < 24 then
                model

            else if abs dx > abs dy then
                queueDir
                    (if dx > 0 then
                        Right

                     else
                        Left
                    )
                    { model | touchFired = True }

            else
                queueDir
                    (if dy > 0 then
                        Down

                     else
                        Up
                    )
                    { model | touchFired = True }

        _ ->
            model


{-| Advance the tweens. Runs on animation frames, not on the game clock.
-}
animate : Float -> Model -> Model
animate dt model =
    { model
        | foodPop = min 1 (model.foodPop + dt / 200)
        , headPulse = model.headPulse + (1 - model.headPulse) * min 1 (dt / 90)
        , particles =
            model.particles
                |> List.map
                    (\p ->
                        { p
                            | life = p.life - dt
                            , x = p.x + p.vx * dt * 0.001
                            , y = p.y + p.vy * dt * 0.001
                        }
                    )
                |> List.filter (\p -> p.life > 0)
    }


{-| One move. Walls and the snake's own body are fatal; the body is checked
whole, tail included, before the tail is popped.
-}
advance : Model -> Model
advance model =
    if not model.alive then
        model

    else
        let
            withAI =
                if model.mode == Demo then
                    let
                        decision =
                            decide (boardOf model)
                    in
                    { model
                        | decision = Just decision
                        , queue = [ decision.dir ]
                        , badge =
                            if model.showVision && Just decision.tier /= model.lastTier then
                                badgeTicks

                            else
                                max 0 (model.badge - 1)
                        , lastTier =
                            if model.showVision then
                                Just decision.tier

                            else
                                Nothing
                    }

                else
                    model

            ( want, rest ) =
                case withAI.queue of
                    d :: tail ->
                        ( d, tail )

                    [] ->
                        ( withAI.dir, [] )

            dir =
                if List.length withAI.snake > 1 && opposite want withAI.dir then
                    withAI.dir

                else
                    want

            head =
                List.head withAI.snake |> Maybe.withDefault { x = 0, y = 0 }

            next =
                step dir head

            moved =
                { withAI | queue = rest, dir = dir }
        in
        if not (inBounds (boardOf moved) next) || Set.member (key next) moved.occupied then
            { moved | alive = False, deaths = moved.deaths + 1 }

        else if moved.food == Just next then
            let
                snake =
                    next :: moved.snake

                occupied =
                    Set.insert (key next) moved.occupied

                ( food, seed ) =
                    spawnFood moved.cols moved.rows occupied moved.seed

                score =
                    moved.score + 1
            in
            { moved
                | snake = snake
                , occupied = occupied
                , food = food
                , seed = seed
                , score = score
                , best = max moved.best score
                , alive = food /= Nothing
                , won = food == Nothing
                , deaths =
                    if food == Nothing then
                        moved.deaths + 1

                    else
                        moved.deaths
                , foodPop = 0
                , headPulse = 1.15
                , particles = moved.particles ++ burst next
            }

        else
            let
                kept =
                    List.take (List.length moved.snake - 1) moved.snake

                dropped =
                    List.drop (List.length moved.snake - 1) moved.snake
            in
            { moved
                | snake = next :: kept
                , occupied =
                    moved.occupied
                        |> (\s -> List.foldl (\p acc -> Set.remove (key p) acc) s dropped)
                        |> Set.insert (key next)
            }


{-| Twenty particles thrown out of the cell where the food was.
-}
burst : Pos -> List Particle
burst pos =
    let
        cx =
            toFloat (pos.x * cellPx) + toFloat cellPx / 2

        cy =
            toFloat (pos.y * cellPx) + toFloat cellPx / 2

        make i =
            let
                angle =
                    turns (toFloat i / 20 + 0.017 * toFloat (modBy 7 i))

                speed =
                    40 + toFloat (modBy 5 (i * 37)) * 25
            in
            { x = cx
            , y = cy
            , vx = cos angle * speed
            , vy = sin angle * speed
            , life = 300 + toFloat (modBy 300 (i * 91))
            }
    in
    List.map make (List.range 0 19)



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ if model.screen == Playing && model.alive then
            Time.every tickMs Tick

          else
            Sub.none
        , Browser.Events.onKeyDown (Decode.map KeyDown (Decode.field "code" Decode.string))
        , Browser.Events.onResize Resize
        , if needsFrames model then
            Browser.Events.onAnimationFrameDelta Frame

          else
            Sub.none
        ]


{-| Only ask for animation frames while something is actually moving.
-}
needsFrames : Model -> Bool
needsFrames model =
    model.screen
        == Playing
        && (not (List.isEmpty model.particles) || model.foodPop < 1 || model.headPulse > 1.001)



-- VIEW


view : Model -> Html Msg
view model =
    case model.screen of
        Menu ->
            viewMenu model

        Playing ->
            viewGame model


viewMenu : Model -> Html Msg
viewMenu model =
    div [ class "screen", id "menu" ]
        [ h1 [] [ text "SNAKE", span [ class "lang" ] [ text "// ELM" ] ]
        , p [ class "sub" ] [ text "Choose your mode" ]
        , div [ class "cards" ]
            [ button [ class "card normal", Html.Events.onClick (Start Normal) ]
                [ span [ class "label" ] [ text "NORMAL" ]
                , span [ class "hint" ] [ text "Tap or use arrow keys" ]
                ]
            , button [ class "card demo", Html.Events.onClick (Start Demo) ]
                [ span [ class "label" ] [ text "DEMO" ]
                , span [ class "hint" ] [ text "Watch the AI" ]
                ]
            ]
        , p [ class "best" ] [ text (bestLabel model.best) ]
        , p [ class "ai-stats" ] [ text model.aiStats ]
        , div [ class "foot" ]
            [ span []
                [ kbd [] [ text "N" ]
                , text " normal · "
                , kbd [] [ text "D" ]
                , text " demo"
                ]
            , a [ href "../" ] [ text "← all snakes" ]
            ]
        ]


bestLabel : Int -> String
bestLabel best =
    if best > 0 then
        "Best: " ++ String.fromInt best

    else
        "Best: —"


viewGame : Model -> Html Msg
viewGame model =
    div [ class "screen", id "game" ]
        [ div [ class "hud" ]
            [ button
                [ class "icon-btn", title "Menu (M)", Html.Events.onClick OpenMenu ]
                [ text "☰" ]
            , Html.Keyed.node "span"
                []
                -- Re-keying on the score restarts the pop animation.
                [ ( String.fromInt model.score
                  , span [ class "pill pop", id "score" ]
                        [ text ("Score: " ++ String.fromInt model.score) ]
                  )
                ]
            , span [ class "pill", id "best" ] [ text (bestLabel model.best) ]
            , button
                [ id "vision-btn"
                , hidden (model.mode /= Demo)
                , classList [ ( "on", model.showVision ) ]
                , Html.Events.onClick ToggleVision
                ]
                [ text
                    ("AI Vision: "
                        ++ (if model.showVision then
                                "on"

                            else
                                "off"
                           )
                    )
                ]
            ]
        , div
            [ class "stage"
            , id "stage"
            , Html.Events.on "touchstart" (touchDecoder "touches" TouchStart)
            , Html.Events.on "touchmove" (touchDecoder "touches" TouchMove)
            , Html.Events.on "touchend" (Decode.succeed TouchEnd)
            , Html.Events.onClick Restart
            ]
            [ Html.Keyed.node "div"
                [ class "board-wrap" ]
                -- Re-keying on each death restarts the shake animation.
                [ ( String.fromInt model.deaths, board model ) ]
            , viewGameOver model
            , div
                [ id "tier-badge"
                , classList [ ( "show", model.showVision && model.badge > 0 ) ]
                , style "color" (tierColor (Maybe.withDefault Fallback model.lastTier))
                ]
                [ text (tierLabel (Maybe.withDefault Fallback model.lastTier)) ]
            ]
        , footer []
            [ span [ id "controls-hint" ] (controlsHint model)
            , a [ href "../" ] [ text "← all snakes" ]
            ]
        ]


controlsHint : Model -> List (Html Msg)
controlsHint model =
    if model.mode == Demo then
        [ kbd [] [ text "V" ], text " AI vision · ", kbd [] [ text "M" ], text " menu" ]

    else
        [ kbd [] [ text "←↑↓→" ]
        , text " / "
        , kbd [] [ text "WASD" ]
        , text " move · "
        , kbd [] [ text "M" ]
        , text " menu · swipe on mobile"
        ]


viewGameOver : Model -> Html Msg
viewGameOver model =
    if model.alive then
        text ""

    else
        div [ id "gameover" ]
            [ text
                (if model.won then
                    "You Win!\nTap to retry"

                 else
                    "Game Over\nTap to retry"
                )
            ]


tierLabel : Tier -> String
tierLabel tier =
    case tier of
        Tier1 ->
            "Tier 1 · Safe Pursuit"

        Tier2 ->
            "Tier 2 · Max Space"

        Fallback ->
            "Fallback · Toward Food"


tierColor : Tier -> String
tierColor tier =
    case tier of
        Tier1 ->
            "#00cec9"

        Tier2 ->
            "#fdcb6e"

        Fallback ->
            "#ff7675"


touchDecoder : String -> (Float -> Float -> Msg) -> Decoder Msg
touchDecoder list toMsg =
    Decode.map2 toMsg
        (Decode.at [ list, "0", "clientX" ] Decode.float)
        (Decode.at [ list, "0", "clientY" ] Decode.float)


board : Model -> Html Msg
board model =
    let
        w =
            model.cols * cellPx

        h =
            model.rows * cellPx

        alpha =
            if model.alive then
                "1"

            else
                "0.4"
    in
    Svg.svg
        [ SA.viewBox ("0 0 " ++ String.fromInt w ++ " " ++ String.fromInt h)
        -- Intrinsic size, so the browser scales it down like a canvas would.
        , SA.width (String.fromInt w)
        , SA.height (String.fromInt h)
        , SA.class
            (if model.alive || model.won then
                "board"

             else
                "board shake"
            )
        , SA.preserveAspectRatio "xMidYMid meet"
        ]
        (Svg.rect [ SA.width "100%", SA.height "100%", SA.fill "#1a1a2e" ] []
            :: gridLines model
            ++ visionLayer model
            ++ [ Svg.g [ SA.opacity alpha ]
                    (List.map bodyCell (List.drop 1 model.snake)
                        ++ foodMark model
                        ++ headMarks model
                    )
               ]
            ++ List.map particleMark model.particles
        )


gridLines : Model -> List (Html Msg)
gridLines model =
    let
        w =
            model.cols * cellPx

        h =
            model.rows * cellPx

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
    List.map vertical (List.range 1 (model.cols - 1))
        ++ List.map horizontal (List.range 1 (model.rows - 1))


{-| The AI's reasoning: reachable region shaded, planned path dashed.
-}
visionLayer : Model -> List (Html Msg)
visionLayer model =
    case ( model.mode, model.showVision, model.decision ) of
        ( Demo, True, Just decision ) ->
            let
                shade p =
                    Svg.rect
                        [ SA.x (String.fromInt (p.x * cellPx + 1))
                        , SA.y (String.fromInt (p.y * cellPx + 1))
                        , SA.width (String.fromInt (cellPx - 2))
                        , SA.height (String.fromInt (cellPx - 2))
                        , SA.fill "#6c5ce7"
                        , SA.opacity "0.15"
                        ]
                        []

                center p =
                    ( toFloat (p.x * cellPx) + toFloat cellPx / 2
                    , toFloat (p.y * cellPx) + toFloat cellPx / 2
                    )

                points =
                    List.map (center >> (\( x, y ) -> String.fromFloat x ++ "," ++ String.fromFloat y))
                        decision.path

                pathLine =
                    if List.length decision.path < 2 then
                        []

                    else
                        [ Svg.polyline
                            [ SA.points (String.join " " points)
                            , SA.fill "none"
                            , SA.stroke "#ff7675"
                            , SA.strokeWidth "2"
                            , SA.strokeOpacity "0.9"
                            , SA.strokeDasharray "6 6"
                            ]
                            []
                        ]
            in
            List.map shade decision.reachable ++ pathLine

        _ ->
            []


bodyCell : Pos -> Html Msg
bodyCell p =
    Svg.rect
        [ SA.x (String.fromInt (p.x * cellPx + 1))
        , SA.y (String.fromInt (p.y * cellPx + 1))
        , SA.width (String.fromInt (cellPx - 2))
        , SA.height (String.fromInt (cellPx - 2))
        , SA.fill "#6c5ce7"
        ]
        []


foodMark : Model -> List (Html Msg)
foodMark model =
    case model.food of
        Nothing ->
            []

        Just p ->
            let
                ease =
                    1 - (1 - model.foodPop) ^ 3
            in
            [ Svg.circle
                [ SA.cx (String.fromFloat (toFloat (p.x * cellPx) + toFloat cellPx / 2))
                , SA.cy (String.fromFloat (toFloat (p.y * cellPx) + toFloat cellPx / 2))
                , SA.r (String.fromFloat (max 0.5 ((toFloat cellPx / 2 - 2) * ease)))
                , SA.fill "#ff7675"
                ]
                []
            ]


headMarks : Model -> List (Html Msg)
headMarks model =
    case List.head model.snake of
        Nothing ->
            []

        Just p ->
            let
                cx =
                    toFloat (p.x * cellPx) + toFloat cellPx / 2

                cy =
                    toFloat (p.y * cellPx) + toFloat cellPx / 2

                size =
                    (toFloat cellPx - 1) * model.headPulse

                off =
                    toFloat cellPx * 0.22

                eyeOffsets =
                    case model.dir of
                        Up ->
                            [ ( -off, -off ), ( off, -off ) ]

                        Down ->
                            [ ( -off, off ), ( off, off ) ]

                        Left ->
                            [ ( -off, -off ), ( -off, off ) ]

                        Right ->
                            [ ( off, -off ), ( off, off ) ]

                eye ( ex, ey ) =
                    Svg.circle
                        [ SA.cx (String.fromFloat (cx + ex))
                        , SA.cy (String.fromFloat (cy + ey))
                        , SA.r (String.fromFloat (max 1.5 (toFloat cellPx * 0.12)))
                        , SA.fill "#0a0a1e"
                        ]
                        []

                glow =
                    if model.alive then
                        [ Svg.rect
                            [ SA.x (String.fromFloat (cx - (toFloat cellPx + 6) / 2))
                            , SA.y (String.fromFloat (cy - (toFloat cellPx + 6) / 2))
                            , SA.width (String.fromInt (cellPx + 6))
                            , SA.height (String.fromInt (cellPx + 6))
                            , SA.fill "#00cec9"
                            , SA.opacity "0.18"
                            ]
                            []
                        ]

                    else
                        []
            in
            glow
                ++ [ Svg.rect
                        [ SA.x (String.fromFloat (cx - size / 2))
                        , SA.y (String.fromFloat (cy - size / 2))
                        , SA.width (String.fromFloat size)
                        , SA.height (String.fromFloat size)
                        , SA.fill "#00cec9"
                        ]
                        []
                   ]
                ++ (if model.alive then
                        List.map eye eyeOffsets

                    else
                        []
                   )


particleMark : Particle -> Html Msg
particleMark p =
    let
        t =
            max 0 (p.life / 600)
    in
    Svg.circle
        [ SA.cx (String.fromFloat p.x)
        , SA.cy (String.fromFloat p.y)
        , SA.r (String.fromFloat (4 * t))
        , SA.fill "#ff7675"
        , SA.opacity (String.fromFloat t)
        ]
        []



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
