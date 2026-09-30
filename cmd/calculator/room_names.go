package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// Each combination is a display name, never an identity or a credential.
var roomAdjectives = [...]string{
	"Agile", "Amber", "Azure", "Bold", "Brave", "Bright", "Brisk", "Calm",
	"Cheerful", "Clever", "Cozy", "Curious", "Daring", "Dazzling", "Eager", "Earnest",
	"Emerald", "Fair", "Fancy", "Fearless", "Fleet", "Fluffy", "Friendly", "Gentle",
	"Gleaming", "Golden", "Graceful", "Grand", "Happy", "Helpful", "Honest", "Jolly",
	"Joyful", "Keen", "Kind", "Lively", "Lucky", "Mellow", "Merry", "Mighty",
	"Nimble", "Noble", "Patient", "Peaceful", "Playful", "Plucky", "Polite", "Quiet",
	"Radiant", "Ready", "Rosy", "Shiny", "Silver", "Smart", "Smooth", "Snowy",
	"Spry", "Steady", "Sunny", "Swift", "Thoughtful", "Valiant", "Warm", "Wise",
	"Able", "Active", "Adventurous", "Airy", "Alert", "Artful", "Auburn", "Bouncy",
	"Breezy", "Brilliant", "Bubbly", "Capable", "Careful", "Caring", "Charcoal", "Charming",
	"Cobalt", "Comfy", "Copper", "Coral", "Courageous", "Creative", "Crisp", "Dapper",
	"Delightful", "Determined", "Dreamy", "Dynamic", "Elegant", "Energetic", "Festive", "Floral",
	"Fortunate", "Fresh", "Frosty", "Generous", "Glowing", "Hardy", "Humble", "Indigo",
	"Inventive", "Jade", "Jaunty", "Jovial", "Lavender", "Loyal", "Luminous", "Magenta",
	"Majestic", "Marvellous", "Mindful", "Modest", "Observant", "Opal", "Peachy", "Pearl",
	"Perky", "Prudent", "Purple", "Resolute", "Resourceful", "Ruby", "Sage", "Serene",
}

var roomNouns = [...]string{
	"Albatross", "Alpaca", "Antelope", "Badger", "Beaver", "Bison", "Butterfly", "Capybara",
	"Chameleon", "Cheetah", "Chipmunk", "Condor", "Crane", "Deer", "Dolphin", "Dove",
	"Dragonfly", "Duck", "Eagle", "Egret", "Elephant", "Falcon", "Finch", "Firefly",
	"Flamingo", "Fox", "Gazelle", "Gecko", "Giraffe", "Heron", "Hummingbird", "Ibex",
	"Ibis", "Jaguar", "Jay", "Kangaroo", "Koala", "Lark", "Lemur", "Lynx",
	"Manatee", "Meerkat", "Narwhal", "Ocelot", "Octopus", "Otter", "Owl", "Panda",
	"Pelican", "Penguin", "Puffin", "Quail", "Quokka", "Rabbit", "Robin", "Seal",
	"Sparrow", "Squirrel", "Swan", "Toucan", "Turtle", "Wallaby", "Whale", "Wren",
	"Aardvark", "Armadillo", "Auk", "Avocet", "Axolotl", "Baboon", "Bandicoot", "Beetle",
	"Beluga", "Bobcat", "Bonobo", "Bumblebee", "Bunting", "Caracal", "Cardinal", "Cassowary",
	"Chickadee", "Chinchilla", "Civet", "Coati", "Cockatoo", "Cormorant", "Cuckoo", "Curlew",
	"Dingo", "Dormouse", "Echidna", "Elk", "Emu", "Ermine", "Ferret", "Frog",
	"Gibbon", "Gopher", "Grouse", "Gull", "Hare", "Hedgehog", "Impala", "Jacana",
	"Jerboa", "Kestrel", "Kingfisher", "Kiwi", "Kookaburra", "Ladybird", "Llama", "Lobster",
	"Loon", "Macaw", "Marmot", "Mongoose", "Nightingale", "Nuthatch", "Okapi", "Osprey",
	"Pangolin", "Parakeet", "Partridge", "Pheasant", "Pika", "Platypus", "Plover", "Porpoise",
}

func randomRoomAlias() (string, error) {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(roomAdjectives)*len(roomNouns))))
	if err != nil {
		return "", err
	}
	i := int(index.Int64())
	return roomAdjectives[i/len(roomNouns)] + " " + roomNouns[i%len(roomNouns)], nil
}

// ensureRoomParticipant assigns a name once per room and owner. Call before
// opening another transaction: production deliberately uses one connection.
func ensureRoomParticipant(ctx context.Context, db *sql.DB, code, owner string) (contracts.RoomParticipant, error) {
	return ensureRoomParticipantWithAlias(ctx, db, code, owner, randomRoomAlias)
}

func ensureRoomParticipantWithAlias(ctx context.Context, db *sql.DB, code, owner string, nextAlias func() (string, error)) (contracts.RoomParticipant, error) {
	participant := roomParticipant(owner, code)
	lookup := func() error {
		return db.QueryRowContext(ctx, `SELECT public_id, alias FROM room_participants
			WHERE room_code = ? AND session_id = ?`, code, owner).Scan(&participant.ID, &participant.Alias)
	}
	if err := lookup(); !errors.Is(err, sql.ErrNoRows) {
		return participant, err
	}
	// Both owner and alias uniqueness are enforced by SQLite. A concurrent call
	// for this owner reads the winner; an alias collision draws another name.
	for attempt := 0; attempt < 128; attempt++ {
		if err := ctx.Err(); err != nil {
			return contracts.RoomParticipant{}, err
		}
		alias, err := nextAlias()
		if err != nil {
			return contracts.RoomParticipant{}, err
		}
		_, err = db.ExecContext(ctx, `INSERT INTO room_participants
			(room_code, session_id, public_id, alias, created_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`, code, owner, participant.ID, alias, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return contracts.RoomParticipant{}, err
		}
		if err := lookup(); !errors.Is(err, sql.ErrNoRows) {
			return participant, err
		}
	}
	return contracts.RoomParticipant{}, errors.New("room name allocation exhausted")
}

// reconcileRoomParticipants upgrades retained room data without publishing
// new events. It is restartable if startup is interrupted between assignments.
func reconcileRoomParticipants(ctx context.Context, db *sql.DB, code string) error {
	rows, err := db.QueryContext(ctx, `SELECT session_id FROM room_events WHERE room_code = ?
		UNION SELECT r.session_id FROM room_reactions r JOIN room_events e ON e.id = r.event_id WHERE e.room_code = ?
		UNION SELECT session_id FROM room_answer_42 WHERE room_code = ?`, code, code, code)
	if err != nil {
		return err
	}
	var owners []string
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			_ = rows.Close()
			return err
		}
		owners = append(owners, owner)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	for _, owner := range owners {
		if _, err := ensureRoomParticipant(ctx, db, code, owner); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE room_events SET alias = (
		SELECT p.alias FROM room_participants p
		WHERE p.room_code = room_events.room_code AND p.session_id = room_events.session_id
	) WHERE room_code = ?`, code)
	return err
}
