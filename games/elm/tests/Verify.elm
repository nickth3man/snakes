port module Verify exposing (main)

{-| Headless check that the Elm demo AI answers exactly like the reference.

`testdata/ai-trace.json` records decisions from the Rust module, which
`ai-parity.mts` checks against the original TypeScript demo controller. This
worker replays those board states through `Ai.decide` and reports any
disagreement. Run it with `verify.mjs`, not in a browser.

-}

import Ai exposing (Tier(..))
import Board exposing (Board, Dir(..), Pos, dirs, key)
import Json.Decode as Decode exposing (Decoder)
import Platform
import Set


type alias Sample =
    { cols : Int
    , rows : Int
    , snake : List Int
    , food : Maybe Int
    , dir : Int
    , tier : Int
    , chosen : Int
    , reachable : Int
    , path : Int
    }


sampleDecoder : Decoder Sample
sampleDecoder =
    Decode.map8 (\c r s f d t ch re -> Sample c r s f d t ch re 0)
        (Decode.field "cols" Decode.int)
        (Decode.field "rows" Decode.int)
        (Decode.field "snake" (Decode.list Decode.int))
        (Decode.field "food" (Decode.nullable Decode.int))
        (Decode.field "dir" Decode.int)
        (Decode.field "tier" Decode.int)
        (Decode.field "chosen" Decode.int)
        (Decode.field "reachable" Decode.int)
        |> Decode.andThen
            (\partial ->
                Decode.map (\p -> { partial | path = p }) (Decode.field "path" Decode.int)
            )


boardOf : Sample -> Board
boardOf sample =
    let
        pos cell =
            { x = modBy sample.cols cell, y = cell // sample.cols }

        snake =
            List.map pos sample.snake
    in
    { cols = sample.cols
    , rows = sample.rows
    , snake = snake
    , occupied = Set.fromList (List.map key snake)
    , food = Maybe.map pos sample.food
    , dir = dirAt sample.dir
    }


dirAt : Int -> Dir
dirAt i =
    List.drop i dirs |> List.head |> Maybe.withDefault Up


dirIndex : Dir -> Int
dirIndex dir =
    List.indexedMap Tuple.pair dirs
        |> List.filter (\( _, d ) -> d == dir)
        |> List.head
        |> Maybe.map Tuple.first
        |> Maybe.withDefault -1


tierIndex : Tier -> Int
tierIndex tier =
    case tier of
        Tier1 ->
            0

        Tier2 ->
            1

        Fallback ->
            2


check : Int -> Sample -> List String
check i sample =
    let
        got =
            Ai.decide (boardOf sample)

        where_ =
            "sample "
                ++ String.fromInt i
                ++ " ("
                ++ String.fromInt sample.cols
                ++ "x"
                ++ String.fromInt sample.rows
                ++ ", len "
                ++ String.fromInt (List.length sample.snake)
                ++ ")"

        complain label actual expected =
            if actual == expected then
                []

            else
                [ where_
                    ++ ": "
                    ++ label
                    ++ " "
                    ++ String.fromInt actual
                    ++ ", want "
                    ++ String.fromInt expected
                ]
    in
    if tierIndex got.tier /= sample.tier then
        complain "tier" (tierIndex got.tier) sample.tier

    else
        complain "dir" (dirIndex got.dir) sample.chosen
            ++ complain "reachable" (List.length got.reachable) sample.reachable
            ++ complain "path" (List.length got.path) sample.path


main : Program Decode.Value () msg
main =
    Platform.worker
        { init =
            \flags ->
                ( ()
                , case Decode.decodeValue (Decode.list sampleDecoder) flags of
                    Err err ->
                        report
                            { checked = 0
                            , tiers = []
                            , problems = [ Decode.errorToString err ]
                            }

                    Ok samples ->
                        report
                            { checked = List.length samples
                            , tiers =
                                List.map
                                    (\t ->
                                        List.length
                                            (List.filter (\s -> s.tier == t) samples)
                                    )
                                    [ 0, 1, 2 ]
                            , problems = List.concat (List.indexedMap check samples)
                            }
                )
        , update = \_ model -> ( model, Cmd.none )
        , subscriptions = \_ -> Sub.none
        }


port report : { checked : Int, tiers : List Int, problems : List String } -> Cmd msg
